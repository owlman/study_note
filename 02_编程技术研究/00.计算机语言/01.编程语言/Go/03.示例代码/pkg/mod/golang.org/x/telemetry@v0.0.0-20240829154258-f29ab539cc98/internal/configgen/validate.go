// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.22

package main

import (
	"errors"
	"fmt"
	"go/version"

	"golang.org/x/mod/semver"
	"golang.org/x/telemetry/internal/chartconfig"
	"golang.org/x/telemetry/internal/telemetry"
)

// ValidateChartConfig checks that a ChartConfig is complete and coherent,
// returning an error describing all problems encountered, or nil.
func ValidateChartConfig(cfg chartconfig.ChartConfig) error {
	var errs []error
	reportf := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}
	if cfg.Title == "" {
		reportf("title must be set")
	}
	if len(cfg.Issue) == 0 {
		reportf("at least one issue is required")
	}
	if cfg.Program == "" {
		reportf("program must be set")
	}
	if cfg.Counter == "" {
		reportf("counter must be set")
	}
	if cfg.Type == "" {
		reportf("type must be set")
	}
	if cfg.Depth < 0 {
		reportf("invalid depth %d: must be non-negative", cfg.Depth)
	}
	if cfg.Depth != 0 && cfg.Type != "stack" {
		reportf("depth can only be set for \"stack\" chart types")
	}
	valid := semver.IsValid
	if telemetry.IsToolchainProgram(cfg.Program) {
		valid = version.IsValid
	}
	if cfg.Version != "" && !valid(cfg.Version) {
		reportf("%q is not a valid version (must be a go version or semver)", cfg.Version)
	}
	return errors.Join(errs...)
}
