// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.22

package main

import (
	_ "embed"
	"reflect"
	"sort"
	"testing"

	"golang.org/x/telemetry/internal/chartconfig"
	"golang.org/x/telemetry/internal/telemetry"
)

func TestGenerate(t *testing.T) {
	defer func(vers map[string][]string) {
		versionsForTesting = vers
	}(versionsForTesting)
	versionsForTesting = map[string][]string{
		"golang.org/toolchain":     {"v0.0.1-go1.21.0.linux-arm", "v0.0.1-go1.20.linux-arm"},
		"golang.org/x/tools/gopls": {"v0.13.0", "v0.14.0", "v0.15.0-pre.1", "v0.15.0"},
	}
	const raw = `
title: Editor Distribution
counter: gopls/editor:{emacs,vim,vscode,other}
description: measure editor distribution for gopls users.
type: partition
issue: https://go.dev/issue/61038
program: golang.org/x/tools/gopls
version: v0.14.0
`
	gcfgs, err := chartconfig.Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	got, err := generate(gcfgs, padding{2, 1, 1, 2, 2})
	if err != nil {
		t.Fatal(err)
	}
	want := telemetry.UploadConfig{
		GOOS:       goos(),
		GOARCH:     goarch(),
		SampleRate: SamplingRate,
		GoVersion:  []string{"go1.20", "go1.21.0"},
		Programs: []*telemetry.ProgramConfig{{
			Name: "golang.org/x/tools/gopls",
			Versions: []string{
				"v0.14.0",
				"v0.15.0-pre.1",
				"v0.15.0",
				"v0.15.1-pre.1",
				"v0.15.1-pre.2",
				"v0.15.1",
				"v0.15.2-pre.1",
				"v0.15.2-pre.2",
				"v0.15.2",
				"v0.16.0-pre.1",
				"v0.16.0-pre.2",
				"v0.16.0",
				"v0.16.1-pre.1",
				"v0.16.1-pre.2",
				"v0.16.1",
				"v1.0.0-pre.1",
				"v1.0.0-pre.2",
				"v1.0.0",
				"v1.0.1-pre.1",
				"v1.0.1-pre.2",
				"v1.0.1",
			},
			Counters: []telemetry.CounterConfig{{
				Name: "gopls/editor:{emacs,vim,vscode,other}",
				Rate: 1.0,
			}},
		}},
	}
	if !reflect.DeepEqual(*got, want) {
		if len(got.Programs) != len(want.Programs) {
			t.Errorf("generate(): got %d programs, want %d", len(got.Programs), len(want.Programs))
		} else {
			for i, gotp := range got.Programs {
				want := *want.Programs[i]
				if !reflect.DeepEqual(*gotp, want) {
					t.Errorf("generate() program #%d =\n%+v\nwant:\n%+v", i, *gotp, want)

				}
			}
		}
		t.Errorf("generate() =\n%+v\nwant:\n%+v", *got, want)
	}
}

func TestByGoVersion_Less(t *testing.T) {
	got := []string{
		"go1.21.0",
		"go1.21rc1",
		"go1.9",
		"go1.9rc1",
		"go1.6",
		"go1.6beta1",
	}
	want := []string{
		"go1.6beta1",
		"go1.6",
		"go1.9rc1",
		"go1.9",
		"go1.21rc1",
		"go1.21.0",
	}
	sort.Sort(byGoVersion(got))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sort.Sort(byGoVersion(got)) = %v, want %v", got, want)
	}
}

func TestContains(t *testing.T) {
	baseline := func() *telemetry.UploadConfig {
		return &telemetry.UploadConfig{
			GOOS:      goos(),
			GOARCH:    goarch(),
			GoVersion: []string{"go1.20", "go1.21.0"},
			Programs: []*telemetry.ProgramConfig{{
				Name: "golang.org/x/tools/gopls",
				Versions: []string{
					"v0.14.0",
					"v0.15.0-pre.1",
					"v0.15.0",
					"v0.15.1-pre.1",
					"v0.15.1-pre.2",
					"v0.15.1",
				},
				Counters: []telemetry.CounterConfig{{
					Name: "gopls/editor:{emacs,vim,vscode,other}",
					Rate: 1.0,
				}},
			}},
		}
	}

	tests := []struct {
		name               string
		outerMut, innerMut func(*telemetry.UploadConfig)
		want               bool
	}{
		{
			"additional arch",
			func(cfg *telemetry.UploadConfig) { cfg.GOARCH = append(cfg.GOARCH, "fake") },
			func(cfg *telemetry.UploadConfig) {},
			false,
		},
		{
			"additional program",
			func(cfg *telemetry.UploadConfig) { cfg.Programs = append(cfg.Programs, new(telemetry.ProgramConfig)) },
			func(cfg *telemetry.UploadConfig) {},
			false,
		},
		{
			"additional counter",
			func(cfg *telemetry.UploadConfig) {
				cfg.Programs[0].Counters = append(cfg.Programs[0].Counters, telemetry.CounterConfig{})
			},
			func(cfg *telemetry.UploadConfig) {},
			false,
		},
		{
			"additional version",
			func(cfg *telemetry.UploadConfig) {
				cfg.Programs[0].Versions = append(cfg.Programs[0].Versions, "v99.99.99")
			},
			func(cfg *telemetry.UploadConfig) {},
			true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outer := baseline()
			test.outerMut(outer)
			inner := baseline()
			test.innerMut(inner)
			if got := contains(outer, inner); got != test.want {
				t.Errorf("contains(...) = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPadVersions(t *testing.T) {
	tests := []struct {
		versions           []string
		prereleasePatterns []string
		padding            padding
		want               []string
	}{
		{
			nil,
			[]string{"pre.1"},
			padding{1, 1, 1, 1, 2},
			[]string{
				"v0.0.1-pre.1",
				"v0.0.1",
				"v0.1.0-pre.1",
				"v0.1.0",
				"v1.0.0-pre.1",
				"v1.0.0",
			},
		},
		{
			[]string{"v0.8.3", "v0.9.1", "v0.9.2", "v1.0.0", "v1.0.1", "v1.0.2-pre.1", "v1.0.2-pre.2", "v1.0.2-pre.3"},
			[]string{"pre.1", "pre.2", "pre.3", "pre.4"},
			padding{2, 1, 2, 2, 2},
			[]string{
				"v0.8.3",
				"v0.9.1",
				"v0.9.2",
				"v1.0.0",
				"v1.0.1",
				"v1.0.2-pre.1",
				"v1.0.2-pre.2",
				"v1.0.2-pre.3",
				"v1.0.2",
				"v1.0.3-pre.1",
				"v1.0.3-pre.2",
				"v1.0.3",
				"v1.1.0-pre.1",
				"v1.1.0-pre.2",
				"v1.1.0",
				"v1.1.1-pre.1",
				"v1.1.1-pre.2",
				"v1.1.1",
				"v1.2.0-pre.1",
				"v1.2.0-pre.2",
				"v1.2.0",
				"v2.0.0-pre.1",
				"v2.0.0-pre.2",
				"v2.0.0",
				"v2.0.1-pre.1",
				"v2.0.1-pre.2",
				"v2.0.1",
				"v2.1.0-pre.1",
				"v2.1.0-pre.2",
				"v2.1.0",
			},
		},
	}

	for _, test := range tests {
		got := padVersions(test.versions, test.prereleasePatterns, test.padding)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("padVersions(%v, %v) =\n%v\nwant:\n%v", test.versions, test.prereleasePatterns, got, test.want)
		}
	}
}
