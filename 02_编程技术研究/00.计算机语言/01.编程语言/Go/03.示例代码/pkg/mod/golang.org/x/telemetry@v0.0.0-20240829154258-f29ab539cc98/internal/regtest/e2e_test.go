// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package regtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/telemetry/counter"
	"golang.org/x/telemetry/internal/config"
	icounter "golang.org/x/telemetry/internal/counter"
	"golang.org/x/telemetry/internal/telemetry"
	"golang.org/x/telemetry/internal/testenv"
)

func TestRunProg(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)
	testenv.MustHaveExec(t)
	prog1 := NewProgram(t, "prog1", func() int {
		fmt.Println("FuncB")
		return 0
	})
	prog2 := NewProgram(t, "prog2", func() int {
		fmt.Println("FuncC")
		return 1
	})
	telemetryDir := t.TempDir()
	if out, err := RunProg(t, telemetryDir, prog1); err != nil || !bytes.Contains(out, []byte("FuncB")) || bytes.Contains(out, []byte("FuncC")) {
		t.Errorf("first RunProg = (%s, %v), want FuncB' and succeed", out, err)
	}
	t.Run("in subtest", func(t *testing.T) {
		if out, err := RunProg(t, telemetryDir, prog2); err == nil || bytes.Contains(out, []byte("FuncB")) || !bytes.Contains(out, []byte("FuncC")) {
			t.Errorf("second RunProg = (%s, %v), want 'FuncC' and fail", out, err)
		}
	})
}

func programIncCounters() int {
	counter.Inc("counter")
	counter.Inc("counter:surprise")
	counter.New("gopls/editor:expected").Inc()
	counter.New("gopls/editor:surprise").Inc()
	counter.NewStack("stack/expected", 1).Inc()
	counter.NewStack("stack-surprise", 1).Inc()
	return 0
}

func TestE2E_off(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)
	testenv.MustHaveExec(t)
	log.Printf("GOOS=%s GOARCH=%s", runtime.GOOS, runtime.GOARCH)

	prog := NewProgram(t, "prog", programIncCounters)
	tests := []struct {
		mode         string // if empty, don't set the mode
		wantLocalDir bool
	}{
		{"", true},
		{"local", true},
		{"on", true},
		{"off", false},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("mode=%s", test.mode), func(t *testing.T) {
			dir := telemetry.NewDir(t.TempDir())
			if test.mode != "" {
				if err := dir.SetMode(test.mode); err != nil {
					t.Fatalf("SetMode failed: %v", err)
				}
			}
			out, err := RunProg(t, dir.Dir(), prog)
			if err != nil {
				t.Fatalf("program failed unexpectedly (%v)\n%s", err, out)
			}
			_, err = os.Stat(dir.LocalDir())
			if err != nil && !os.IsNotExist(err) {
				t.Fatalf("os.Stat(%q): unexpected error: %v", dir.LocalDir(), err)
			}
			if gotLocalDir := err == nil; gotLocalDir != test.wantLocalDir {
				t.Errorf("got /local dir: %v, want %v; out:\n%s", gotLocalDir, test.wantLocalDir, string(out))
			}
		})
	}
}

func TestE2E(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)
	testenv.MustHaveExec(t)
	programIncCounters := NewProgram(t, "prog", programIncCounters)
	telemetryDir := t.TempDir()
	goVers, progPath, progVers := ProgramInfo(t)

	out, err := RunProg(t, telemetryDir, programIncCounters)
	if err != nil {
		t.Fatalf("program failed unexpectedly (%v)\n%s", err, out)
	}

	// TODO: retrieve config through a module proxy so we test internal/configstore code path.
	cfg := &telemetry.UploadConfig{
		GOOS:      []string{runtime.GOOS},
		GOARCH:    []string{runtime.GOARCH},
		GoVersion: []string{goVers},
		Programs: []*telemetry.ProgramConfig{
			{
				Name:     progPath,
				Versions: []string{progVers},
				Counters: []telemetry.CounterConfig{
					{Name: "counter", Rate: 1},
					{Name: "gopls/editor:{expected, other}", Rate: 1},
				},
				Stacks: []telemetry.CounterConfig{
					{Name: "stack/expected", Rate: 1, Depth: 1},
				},
			},
		},
	}

	// TODO: check if weekday file exists.

	// TODO: test upload path.
	//     - change the global clock (maybe internal/clock package?)
	//     - start an upload server
	//     - Run(t, telemetryDir, func() int { upload.Run(...) })
	//     - check if the upload server received expected data
	//     - check if the local and upload directories in the expected state

	uploaded, notUploaded, err := parseCounters(cfg, telemetryDir)
	if err != nil {
		t.Fatalf("failed to simulate upload: %v", err)
	}

	wantUpload := map[string]uint64{
		"counter":               1,
		"gopls/editor:expected": 1,
		"stack/expected\n":      1, // prefix of the stack counter name + "\n". see parseCounters.
	}
	testCounterUploadStatus(t, uploaded, wantUpload, false)

	wantNotUpload := map[string]uint64{
		"counter:surprise":      1,
		"gopls/editor:surprise": 1,
		"stack-surprise\n":      1, // prefix of the stack counter name + "\n". see parseCounters.
	}
	testCounterUploadStatus(t, notUploaded, wantNotUpload, true)
}

// testCounterUploadStatus checks if got and want counter maps match.
// If allowUnexpected is true, it checks if got is a superset of want.
func testCounterUploadStatus(t *testing.T, got, want map[string]uint64, allowUnexpected bool) {
	t.Helper()

	seen := got
	if allowUnexpected {
		// filter out unexpected counters from got, for comparison.
		m := make(map[string]uint64, len(want))
		for k, v := range got {
			if _, ok := want[k]; ok {
				m[k] = v
			}
		}
		seen = m
	}
	if !reflect.DeepEqual(seen, want) {
		// TODO(hyangah) implement diff or copy Go project's internal/diff for pretty printing of diff.
		// Or, move internal/regtest to the godev module where we can depend on go-cmp.
		t.Errorf("unmet expectation:\ngot %v\nwant %v", stringify(got), stringify(want))
	}
}

func stringify(a any) string {
	encoded, err := json.MarshalIndent(a, "\t", " ")
	if err != nil {
		return fmt.Sprintf("unmarshallable - %v", err)
	}
	return string(encoded)
}

// parseCounters reads all counter files in the local telemetry directory, and
// returns all observed counters grouped by whether the counter names are included
// in the specified configuration.
// For simplicity in the comparison code, the returned maps represent a stack counter
// with its counter name prefix and "\n". For example, if there are "stackcounter\npkg.F:..."
// and "stackcounter\npkg.G:..", "stackcounter\n" will hold the sum of those counters.
func parseCounters(uc *telemetry.UploadConfig, telemetryDir string) (uploadable, notUploadable map[string]uint64, _ error) {
	cfg := config.NewConfig(uc)
	localDir := filepath.Join(telemetryDir, "local")
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return nil, nil, err
	}
	uploadable, notUploadable = make(map[string]uint64), make(map[string]uint64)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".count" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(localDir, entry.Name()))
		if err != nil { // ignore unreadable file.
			continue
		}
		// TODO(hyangah): how about exposing "Parse" to public for testing? (i.e. countertest.Parse)?
		parsed, err := icounter.Parse(entry.Name(), data)
		if err != nil { // ignore unparsable file
			continue
		}
		// The following is temporary until the upload package implements the exact same logic.
		// TODO(hyangah): replace with the shared logic between the uploader and the local viewer.
		maybeUploadable := true &&
			cfg.HasGOOS(parsed.Meta["GOOS"]) &&
			cfg.HasGOARCH(parsed.Meta["GOARCH"]) &&
			cfg.HasGoVersion(parsed.Meta["GoVersion"]) &&
			cfg.HasProgram(parsed.Meta["Program"]) &&
			cfg.HasVersion(parsed.Meta["Program"], parsed.Meta["Version"])

		for k, v := range parsed.Count {
			counterPrefix, _, isStackCounter := strings.Cut(k, "\n")
			isUploadable := maybeUploadable
			key := k
			if isStackCounter {
				isUploadable = isUploadable && cfg.HasStack(parsed.Meta["Program"], counterPrefix)
				key = counterPrefix + "\n"
			} else {
				isUploadable = isUploadable && cfg.HasCounter(parsed.Meta["Program"], k)
			}
			if isUploadable {
				uploadable[key] = uploadable[key] + v
			} else {
				notUploadable[key] = notUploadable[key] + v
			}
		}
	}
	return uploadable, notUploadable, nil
}
