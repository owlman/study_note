// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package telemetrymode manages the telemetry mode file.
package telemetry

import (
	"os"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	defaultDirMissing := false
	if _, err := os.UserConfigDir(); err != nil {
		defaultDirMissing = true
	}
	if defaultDirMissing {
		if Default.LocalDir() != "" || Default.UploadDir() != "" || Default.ModeFile() != "" {
			t.Errorf("DefaultSetting: (%q, %q, %q), want empty LocalDir/UploadDir/ModeFile", Default.LocalDir(), Default.UploadDir(), Default.ModeFile())
		}
	} else {
		if Default.LocalDir() == "" || Default.UploadDir() == "" || Default.ModeFile() == "" {
			t.Errorf("DefaultSetting: (%q, %q, %q), want non-empty LocalDir/UploadDir/ModeFile", Default.LocalDir(), Default.UploadDir(), Default.ModeFile())
		}
	}
}

func TestTelemetryModeWithNoModeConfig(t *testing.T) {
	tests := []struct {
		dir  Dir
		want string
	}{
		{NewDir(t.TempDir()), "local"},
		{Dir{}, "off"},
	}
	for _, tt := range tests {
		if got, _ := tt.dir.Mode(); got != tt.want {
			t.Errorf("Dir{modefile=%q}.Mode() = %v, want %v", tt.dir.ModeFile(), got, tt.want)
		}
	}
}

func TestSetMode(t *testing.T) {
	tests := []struct {
		in      string
		wantErr bool // want error when setting.
	}{
		{"on", false},
		{"off", false},
		{"local", false},
		{"https://mytelemetry.com", true},
		{"http://insecure.com", true},
		{"bogus", true},
		{"", true},
	}
	for _, tt := range tests {
		t.Run("mode="+tt.in, func(t *testing.T) {
			dir := NewDir(t.TempDir())
			setErr := dir.SetMode(tt.in)
			if (setErr != nil) != tt.wantErr {
				t.Fatalf("Set() error = %v, wantErr %v", setErr, tt.wantErr)
			}
			if setErr != nil {
				return
			}
			if got, _ := dir.Mode(); got != tt.in {
				t.Errorf("LookupMode() = %q, want %q", got, tt.in)
			}
		})
	}
}

func TestMode(t *testing.T) {
	tests := []struct {
		in       string
		wantMode string
		wantTime time.Time
	}{
		{"on", "on", time.Time{}},
		{"on 2023-09-26", "on", time.Date(2023, time.September, 26, 0, 0, 0, 0, time.UTC)},
		{"off", "off", time.Time{}},
		{"local", "local", time.Time{}},
	}
	for _, tt := range tests {
		t.Run("mode="+tt.in, func(t *testing.T) {
			dir := NewDir(t.TempDir())
			if err := os.WriteFile(dir.ModeFile(), []byte(tt.in), 0666); err != nil {
				t.Fatal(err)
			}
			// Note: the checks below intentionally do not use time.Equal:
			// we want this exact representation of time.
			if gotMode, gotTime := dir.Mode(); gotMode != tt.wantMode || gotTime != tt.wantTime {
				t.Errorf("ModeFilePath(contents=%s).Mode() = %q, %v, want %q, %v", tt.in, gotMode, gotTime, tt.wantMode, tt.wantTime)
			}
		})
	}
}
