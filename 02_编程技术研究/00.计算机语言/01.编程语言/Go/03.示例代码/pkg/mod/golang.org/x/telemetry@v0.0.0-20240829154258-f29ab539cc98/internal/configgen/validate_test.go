// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.22

package main

import (
	"strings"
	"testing"

	"golang.org/x/telemetry/internal/chartconfig"
)

func TestLoadedChartsAreValid(t *testing.T) {
	// Test that we can actually load the chart config.
	charts, err := chartconfig.Load()
	if err != nil {
		t.Errorf("Load() failed: %v", err)
	}
	for i, chart := range charts {
		if err := ValidateChartConfig(chart); err != nil {
			t.Errorf("Chart %d is invalid: %v", i, err)
		}
	}
}

func TestValidateOK(t *testing.T) {
	// A minimally valid chart config.
	const input = `
title: Editor Distribution
counter: gopls/editor:{emacs,vim,vscode,other}
type: partition
issue: https://go.dev/issue/12345
program: golang.org/x/tools/gopls
`
	records, err := chartconfig.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("Parse(%q) returned %d records, want exactly 1", input, len(records))
	}
	if err := ValidateChartConfig(records[0]); err != nil {
		t.Errorf("Validate(%q) = %v, want nil", input, err)
	}
}

func TestValidate(t *testing.T) {
	tests := map[string][]string{ // input -> want errors
		// validation of mandatory fields
		"description:bar": {"title", "program", "issue", "counter", "type"},

		// validation of semver intervals
		"version:1.2.3.4": {"semver"},

		// valid of stack configuration
		"depth:-1": {"non-negative", "stack"},
	}

	for input, wantErrs := range tests {
		records, err := chartconfig.Parse([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		if len(records) != 1 {
			t.Fatalf("Parse(%q) returned %d records, want exactly 1", input, len(records))
		}
		err = ValidateChartConfig(records[0])
		if err == nil {
			t.Fatalf("Validate(%q) succeeded unexpectedly", input)
		}
		errs := err.Error()
		for _, want := range wantErrs {
			if !strings.Contains(errs, want) {
				t.Errorf("Validate(%q) = %v, want containing %q", input, err, want)
			}
		}
	}
}
