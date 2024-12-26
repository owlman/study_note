// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package telemetry_test

import (
	"fmt"
	"path"
	"runtime/debug"
	"testing"

	"golang.org/x/telemetry/internal/telemetry"
)

func TestProgramInfo_ProgramVersion(t *testing.T) {
	tests := []struct {
		path    string
		version string
		want    string
	}{
		{
			path:    "golang.org/x/tools/gopls",
			version: "(devel)",
			want:    "devel",
		},
		{
			path:    "golang.org/x/tools/gopls",
			version: "",
			want:    "",
		},
		{
			path:    "golang.org/x/tools/gopls",
			version: "v0.14.0-pre.1",
			want:    "v0.14.0-pre.1",
		},
		{
			path:    "golang.org/x/tools/gopls",
			version: "v0.0.0-20231207172801-3c8b0df0c3fd",
			want:    "devel",
		},
		{
			path:    "cmd/go",
			version: "",
			want:    "go1.23.0", // hard-coded below
		},
		{
			path:    "cmd/compile",
			version: "",
			want:    "go1.23.0", // hard-coded below
		},
	}
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("cannot use debug.ReadBuildInfo")
	}

	for _, tt := range tests {
		name := fmt.Sprintf("%s@%s", path.Base(tt.path), tt.version)
		t.Run(name, func(t *testing.T) {
			in := *buildInfo
			in.GoVersion = "go1.23.0"
			in.Path = tt.path
			in.Main.Version = tt.version
			_, _, got := telemetry.ProgramInfo(&in)
			if got != tt.want {
				t.Errorf("program version = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProgramInfo_GoVersion(t *testing.T) {
	tests := []struct {
		goVersion  string
		wantGoVers string
	}{
		{
			"go1.23.0-bigcorp",
			"devel",
		},
		{
			"go1.23.0",
			"go1.23.0",
		},
		{
			"devel go1.24-0d6bb68f48 Thu Jul 25 23:27:41 2024 -0600",
			"devel",
		},
		{
			"go1.23rc2 X:aliastypeparams",
			"devel",
		},
	}
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("cannot use debug.ReadBuildInfo")
	}

	for _, tt := range tests {
		t.Run(tt.goVersion, func(t *testing.T) {
			in := *buildInfo
			in.GoVersion = tt.goVersion
			in.Path = "cmd/go"
			in.Main.Version = tt.goVersion
			gotGoVers, _, gotProgVers := telemetry.ProgramInfo((&in))
			if gotGoVers != tt.wantGoVers {
				t.Errorf("go version = %q, want %q", gotGoVers, tt.wantGoVers)
			}
			if gotProgVers != tt.wantGoVers {
				t.Errorf("program version = %q, want %q", gotProgVers, tt.wantGoVers)
			}
		})
	}
}
