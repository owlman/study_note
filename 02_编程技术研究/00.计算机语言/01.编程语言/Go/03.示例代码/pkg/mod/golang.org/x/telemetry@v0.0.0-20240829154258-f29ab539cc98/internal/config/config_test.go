// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/telemetry/internal/telemetry"
)

func TestConfig(t *testing.T) {
	f, err := os.Open(filepath.FromSlash("../../config/config.json"))
	if os.IsNotExist(err) {
		t.Skip("config file not found")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var cfg telemetry.UploadConfig
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err := d.Decode(&cfg); err != nil {
		t.Fatal(err)
	}
}

func TestInternalConfig(t *testing.T) {
	got, err := ReadConfig("testdata/config.json")
	if err != nil {
		t.Fatal(err)
	}
	wantGOOS := []string{"linux", "darwin"}
	wantGOARCH := []string{"amd64", "arm64"}
	wantGoVersion := []string{"go1.20", "go1.20.1"}
	wantPrograms := []string{"golang.org/x/tools/gopls", "cmd/go"}
	wantVersions := [][2]string{
		{"golang.org/x/tools/gopls", "v0.10.1"},
		{"golang.org/x/tools/gopls", "v0.11.0"},
	}
	wantCounters := [][2]string{
		{"golang.org/x/tools/gopls", "editor:emacs"},
		{"golang.org/x/tools/gopls", "editor:vim"},
		{"golang.org/x/tools/gopls", "editor:vscode"},
		{"golang.org/x/tools/gopls", "editor:other"},
		{"cmd/go", "go/buildcache/miss:0"},
		{"cmd/go", "go/buildcache/miss:1"},
		{"cmd/go", "go/buildcache/miss:10"},
		{"cmd/go", "go/buildcache/miss:100"},
	}
	wantPrefix := [][2]string{
		{"golang.org/x/tools/gopls", "editor"},
		{"cmd/go", "go/buildcache/miss"},
	}

	for _, w := range wantGOOS {
		if !got.HasGOOS(w) {
			t.Errorf("got.HasGOOS(%s) = false: want true", w)
		}
	}
	for _, w := range wantGOARCH {
		if !got.HasGOARCH(w) {
			t.Errorf("got.HasGOARCH(%s) = false: want true", w)
		}
	}
	for _, w := range wantGoVersion {
		if !got.HasGoVersion(w) {
			t.Errorf("got.HasGoVersion(%s) = false: want true", w)
		}
	}
	for _, w := range wantPrograms {
		if !got.HasProgram(w) {
			t.Errorf("got.HasProgram(%s) = false: want true", w)
		}
	}
	for _, w := range wantVersions {
		if !got.HasVersion(w[0], w[1]) {
			t.Errorf("got.HasVersion(%s, %s) = false: want true", w[0], w)
		}
	}
	for _, w := range wantCounters {
		if !got.HasCounter(w[0], w[1]) {
			t.Errorf("got.HasCounter(%s, %s) = false: want true", w[0], w[1])
		}
	}
	for _, w := range wantPrefix {
		if !got.HasCounterPrefix(w[0], w[1]) {
			t.Errorf("got.HasCounterPrefix(%s, %s) = false: want true", w[0], w[1])
		}
	}
}
