// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run . -w

//go:build go1.22

// Package configgen generates the upload config file stored in the config.json
// file of golang.org/x/telemetry/config based on the chartconfig stored in
// config.txt.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/version"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
	"golang.org/x/telemetry/internal/chartconfig"
	"golang.org/x/telemetry/internal/telemetry"
)

var (
	write = flag.Bool("w", false, "if set, write the config file; otherwise, print to stdout")
	force = flag.Bool("f", false, "if set, force the write of the config file even if the current content is still valid")

	// SamplingRate is the fraction of otherwise uploadable reports that will be uploaded
	SamplingRate = 1.0
)

func main() {
	flag.Parse()

	gcfgs, err := chartconfig.Load()
	if err != nil {
		log.Fatal(err)
	}

	// The padding heuristics below are based on the example of gopls.
	//
	// The goal is to pad enough versions for a quarter.
	uCfg, err := generate(gcfgs, padding{
		// 6 releases into the future translates to approximately three months for gopls.
		releases: 6,
		// We may release gopls 1.0, but won't release 2.0 in a three month timespan!
		maj: 1,
		// We don't usually do more than one minor release a month.
		majmin: 3,
		// Since golang/go#55267, which committed to adhering to semver, gopls
		// hasn't had more than 5 patches per minor version.
		patch: 6,
		// Gopls has never had more than 4 prereleases.
		pre: 4,
	})

	if err != nil {
		log.Fatal(err)
	}
	cfgJSON, err := json.MarshalIndent(uCfg, "", "\t")
	if err != nil {
		log.Fatal(err)
	}

	if !*write {
		fmt.Println(string(cfgJSON))
		os.Exit(0)
	}

	configFile, err := configFile()
	if err != nil {
		log.Fatalf("finding config file: %v", err)
	}

	if !*force {
		currentCfg, err := readConfig(configFile)
		if err != nil {
			log.Fatal(err)
		}
		// Guarantee that we have enough padding to do two patches releases tomorrow.
		minCfg, err := generate(gcfgs, padding{
			releases: 2,
			maj:      1,
			majmin:   1, // we're not ever going to do more than one major/minor release in a day
			patch:    2,
			pre:      2, // in a single day, we wouldn't prep more than two prereleases per version
		})
		if err != nil {
			log.Fatal(err)
		}
		if contains(currentCfg, minCfg) {
			fmt.Fprintln(os.Stderr, "not writing the config file as it is still valid; use -f to force")
			os.Exit(0)
		}
	}
	if err := os.WriteFile(configFile, cfgJSON, 0666); err != nil {
		log.Fatal(err)
	}
}

// configFile returns the path to the x/telemetry/config config.json file in
// this repo.
//
// The file must already exist: this won't be a valid location if running from
// the module cache; this functionality only works when executed from the
// telemetry repo.
func configFile() (string, error) {
	out, err := exec.Command("go", "list", "-f", "{{.Dir}}", "golang.org/x/telemetry/internal/configgen").Output()
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(string(out))
	configFile := filepath.Join(dir, "..", "..", "config", "config.json")
	if _, err := os.Stat(configFile); err != nil {
		return "", err
	}
	return configFile, nil
}

func readConfig(file string) (*telemetry.UploadConfig, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %v", err)
	}
	cfg := new(telemetry.UploadConfig)
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config file: %v", err)
	}
	return cfg, nil
}

// generate computes the upload config from chart configs and module
// information, returning the resulting formatted JSON.
func generate(gcfgs []chartconfig.ChartConfig, padding padding) (*telemetry.UploadConfig, error) {
	ucfg := &telemetry.UploadConfig{
		GOOS:   goos(),
		GOARCH: goarch(),
		// the probability of uploading a report
		SampleRate: SamplingRate,
	}
	var err error
	ucfg.GoVersion, err = goVersions()
	if err != nil {
		return nil, fmt.Errorf("querying go info: %v", err)
	}

	for i, r := range gcfgs {
		if err := ValidateChartConfig(r); err != nil {
			// TODO(rfindley): this is a poor way to identify the faulty record. We
			// should probably store position information in the ChartConfig.
			return nil, fmt.Errorf("chart config #%d (%q): %v", i, r.Title, err)
		}
	}

	var (
		programs    = make(map[string]*telemetry.ProgramConfig) // package path -> config
		minVersions = make(map[string]string)                   // package path -> min version required, or "" for all
	)
	for _, gcfg := range gcfgs {
		pcfg := programs[gcfg.Program]
		if pcfg == nil {
			pcfg = &telemetry.ProgramConfig{
				Name: gcfg.Program,
			}
			programs[gcfg.Program] = pcfg
			minVersions[gcfg.Program] = gcfg.Version
		}
		minVersions[gcfg.Program] = minVersion(minVersions[gcfg.Program], gcfg.Version)
		ccfg := telemetry.CounterConfig{
			Name:  gcfg.Counter,
			Rate:  1.0, // TODO(rfindley): how should rate be configured?
			Depth: gcfg.Depth,
		}
		if gcfg.Depth > 0 {
			pcfg.Stacks = append(pcfg.Stacks, ccfg)
		} else {
			pcfg.Counters = append(pcfg.Counters, ccfg)
		}
	}

	for _, p := range programs {
		minVersion := minVersions[p.Name]

		// Collect eligible program versions. If p is a toolchain tool (cmd/go,
		// cmd/compile, etc), these come out of the Go versions queried above.
		// Otherwise, they come from the proxy.
		//
		// In both of these cases, the versions should be valid, but we verify
		// anyway as otherwise the version comparison is meaningless.
		// (and in fact, there is an invalid go1.9.2rc2 version in the proxy)
		if telemetry.IsToolchainProgram(p.Name) {
			// Note: no need to pad versions for toolchain programs, since the
			// toolchain is released infrequently.
			// (and in any case, version padding only works for semantic versions)
			for _, v := range ucfg.GoVersion {
				if !version.IsValid(v) {
					// The proxy toolchain versions list go1.9.2rc2, which is invalid.
					// Skip it.
					continue
				}

				if minVersion == "" || version.Compare(minVersion, v) <= 0 {
					p.Versions = append(p.Versions, v)
				}
			}
		} else {
			versions, err := listProxyVersions(p.Name)
			if err != nil {
				return nil, fmt.Errorf("listing versions for %q: %v", p.Name, err)
			}
			// Filter proxy versions in place.
			i := 0
			for _, v := range versions {
				if !semver.IsValid(v) {
					return nil, fmt.Errorf("invalid semver %q returned from proxy for %q", v, p.Name)
				}
				if minVersion == "" || semver.Compare(minVersion, v) <= 0 {
					versions[i] = v
					i++
				}
			}
			p.Versions = padVersions(versions[:i], prereleasesForProgram(p.Name), padding)
		}
		ucfg.Programs = append(ucfg.Programs, p)
	}
	sort.Slice(ucfg.Programs, func(i, j int) bool {
		return ucfg.Programs[i].Name < ucfg.Programs[j].Name
	})

	return ucfg, nil
}

// contains reports whether outer contains all program versions of inner, and
// is otherwise equivalent to inner.
func contains(outer, inner *telemetry.UploadConfig) bool {
	if !slices.Equal(outer.GOARCH, inner.GOARCH) {
		return false
	}
	if !slices.Equal(outer.GOOS, inner.GOOS) {
		return false
	}
	if !slices.Equal(outer.GoVersion, inner.GoVersion) {
		return false
	}

	for _, pi := range inner.Programs {
		i := slices.IndexFunc(outer.Programs, func(po *telemetry.ProgramConfig) bool {
			return po.Name == pi.Name
		})
		if i < 0 {
			return false
		}
		po := outer.Programs[i]
		if !sliceContains(po.Versions, pi.Versions) {
			return false
		}
		if !slices.Equal(po.Counters, pi.Counters) {
			return false
		}
		if !slices.Equal(po.Stacks, pi.Stacks) {
			return false
		}
	}
	for _, po := range outer.Programs {
		if !slices.ContainsFunc(inner.Programs, func(pi *telemetry.ProgramConfig) bool {
			return pi.Name == po.Name
		}) {
			return false
		}
	}
	return true
}

func sliceContains[T comparable](outer, inner []T) bool {
	m := toMap(outer)
	for _, v := range inner {
		if !m[v] {
			return false
		}
	}
	return true
}

func toMap[T comparable](s []T) map[T]bool {
	m := make(map[T]bool)
	for _, v := range s {
		m[v] = true
	}
	return m
}

// prereleasesForProgram returns the set of prereleases to use for the provided
// program. We may need to customize this for the conventions of different
// programs.
func prereleasesForProgram(program string) []string {
	// Surely eight prereleases is enough for any program... :)
	return []string{"pre.1", "pre.2", "pre.3", "pre.4", "pre.5", "pre.6", "pre.7", "pre.8"}
}

// minVersion returns the lesser semantic version of v1 and v2.
//
// As a special case, the empty string is treated as an absolute minimum
// (empty => all versions are greater).
func minVersion(v1, v2 string) string {
	if v1 == "" || v2 == "" {
		return ""
	}
	if semver.Compare(v1, v2) > 0 {
		return v2
	}
	return v1
}

// goos returns a sorted slice of known GOOS values.
func goos() []string {
	var gooses []string
	for goos := range knownOS {
		gooses = append(gooses, goos)
	}
	sort.Strings(gooses)
	return gooses
}

// goarch returns a sorted slice of known GOARCH values.
func goarch() []string {
	var arches []string
	for arch := range knownArch {
		arches = append(arches, arch)
	}
	sort.Strings(arches)
	return arches
}

// goInfo queries the proxy for information about go distributions, including
// versions, GOOS, and GOARCH values.
func goVersions() ([]string, error) {
	// Trick: read Go distribution information from the module versions of
	// golang.org/toolchain. These define the set of valid toolchains, and
	// therefore are a reasonable source for version information.
	//
	// A more authoritative source for this information may be
	// https://go.dev/dl?mode=json&include=all.
	proxyVersions, err := listProxyVersions("golang.org/toolchain")
	if err != nil {
		return nil, fmt.Errorf("listing toolchain versions: %v", err)
	}
	var goVersionRx = regexp.MustCompile(`^-(go.+)\.[^.]+-[^.]+$`)
	verSet := make(map[string]struct{})
	for _, v := range proxyVersions {
		pre := semver.Prerelease(v)
		match := goVersionRx.FindStringSubmatch(pre)
		if match == nil {
			return nil, fmt.Errorf("proxy version %q does not match prerelease regexp %q", v, goVersionRx)
		}
		verSet[match[1]] = struct{}{}
	}
	var vers []string
	for v := range verSet {
		vers = append(vers, v)
	}
	sort.Sort(byGoVersion(vers))
	return vers, nil
}

type byGoVersion []string

func (vs byGoVersion) Len() int      { return len(vs) }
func (vs byGoVersion) Swap(i, j int) { vs[i], vs[j] = vs[j], vs[i] }
func (vs byGoVersion) Less(i, j int) bool {
	cmp := version.Compare(vs[i], vs[j])
	if cmp != 0 {
		return cmp < 0
	}
	// To ensure that we have a stable sort, order equivalent Go versions lexically.
	return vs[i] < vs[j]
}

// versionsForTesting contains versions to use for testing, rather than
// querying the proxy.
var versionsForTesting map[string][]string

// listProxyVersions queries the Go module mirror for published versions of the
// given modulePath.
//
// modulePath must be lower-case (or already escaped): this function doesn't do
// any escaping of upper-cased letters, as is required by the proxy prototol
// (https://go.dev/ref/mod#goproxy-protocol).
func listProxyVersions(modulePath string) ([]string, error) {
	if vers, ok := versionsForTesting[modulePath]; ok {
		return vers, nil
	}
	cmd := exec.Command("go", "list", "-m", "--versions", modulePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing versions: %v (stderr: %v)", err, stderr.String())
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) == 0 {
		return nil, fmt.Errorf("invalid version list output: %q", string(out))
	}
	return fields[1:], nil
}

// padding defines constraints on additional versions to pad.
//
// These constraints help restrict version padding to "reasonable" versions,
// based on heuristics such as "we never do more than 3 prereleases for a given
// version" or "we never have more than 5 patch versions" or "we can't do more
// than 10 total releases over that time period". See the field documentation
// for details.
type padding struct {
	releases int // bounds on the total number of releases
	maj      int // bounds the number of new major versions
	majmin   int // bounds the number of new major+minor versions
	patch    int // bounds the number of new patch versions
	pre      int // the number of prereleases to pad per release
}

// padVersions pads the existing version list with potential next versions, so
// that we don't have to wait an additional day to start getting reports for a
// newly tagged version.
//
// The prereleases argument may be supplied to provide a set of potential
// prerelease candidates. For example, if the program releases prereleases of
// the form "-pre.N", prereleases should be {"pre.1", "pre.2", ...}. For each
// potential next release version, the next two prerelease versions will be
// selected out of the provided set of prereleases.
func padVersions(versions []string, prereleasePatterns []string, padding padding) []string {
	versions = slices.Clone(versions)
	semver.Sort(versions)

	latestRelease := "v0.0.0"
	all := make(map[string]bool) // for de-duplicating padded versions
	for _, v := range versions {
		cv := semver.Canonical(v)
		all[cv] = true
		if semver.Prerelease(cv) == "" && semver.Compare(latestRelease, cv) < 0 {
			latestRelease = cv
		}
	}

	parsedLatest, ok := parseSemver(latestRelease)
	if !ok {
		// "can't happen", since the latest release version should always be canonical.
		panic(fmt.Sprintf("unable to parse latest release version %q", latestRelease))
	}

	// Pad the latest version only.
	//
	// This assumes that the program in question doesn't patch older releases
	// (as is the case with gopls). If that assumption ever changes, we may need
	// to apply padding to older versions as well.
	versionsToPad := []semversion{parsedLatest}

	var maj, min, patch int
	for _, toPad := range versionsToPad {
		for majPadding := 0; majPadding <= padding.maj; majPadding++ {
			maj = toPad.major + majPadding
			for minPadding := 0; minPadding+majPadding <= padding.majmin; minPadding++ {
				if majPadding == 0 {
					min = toPad.minor + minPadding
				} else {
					min = minPadding
				}
				for patchPadding := 0; patchPadding <= padding.patch; patchPadding++ {
					releases := majPadding + minPadding + patchPadding
					if releases == 0 || releases > padding.releases {
						continue
					}
					if majPadding == 0 && minPadding == 0 {
						patch = toPad.patch + patchPadding
					} else {
						patch = patchPadding
					}

					v := fmt.Sprintf("v%d.%d.%d", maj, min, patch)
					if all[v] {
						// This guard is future proofing: we may have seen this version
						// before if we are ever padding something other than the latest
						// version.
						continue
					}
					versions = append(versions, v)

					// We may already have prereleases at this version. Don't pad
					// additional prereleases, under the assumption that we don't
					// typically have more than padding.pre prereleases.
					nextPrerelease := 0
					for i, patt := range prereleasePatterns {
						pre := fmt.Sprintf("%s-%s", v, patt)
						if all[pre] {
							nextPrerelease = i + 1
						}
					}
					for i := nextPrerelease; i < len(prereleasePatterns) && i < padding.pre; i++ {
						pre := fmt.Sprintf("%s-%s", v, prereleasePatterns[i])
						versions = append(versions, pre)
					}
				}
			}
		}
	}

	semver.Sort(versions)
	return versions
}

// version is a parsed semantic version.
type semversion struct {
	major, minor, patch int
	pre                 string
}

// parseSemver attempts to parse semver components out of the provided semver
// v. If v is not valid semver in canonical form, parseSemver returns _, _, _,
// _, false.
func parseSemver(v string) (_ semversion, ok bool) {
	var parsed semversion
	v, parsed.pre, _ = strings.Cut(v, "-")
	if _, err := fmt.Sscanf(v, "v%d.%d.%d", &parsed.major, &parsed.minor, &parsed.patch); err == nil {
		ok = true
	}
	return parsed, ok
}
