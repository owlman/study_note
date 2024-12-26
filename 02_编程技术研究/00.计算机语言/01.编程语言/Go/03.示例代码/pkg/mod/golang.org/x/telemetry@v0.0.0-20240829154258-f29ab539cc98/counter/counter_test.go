// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package counter_test

import (
	"os"
	"os/exec"
	"testing"

	"golang.org/x/telemetry/counter"
	"golang.org/x/telemetry/internal/telemetry"
	"golang.org/x/telemetry/internal/testenv"
)

const telemetryDirEnvVar = "_COUNTER_TEST_TELEMETRY_DIR"

func TestMain(m *testing.M) {
	if dir := os.Getenv(telemetryDirEnvVar); dir != "" {
		// Run for TestOpenAPIMisuse.
		telemetry.Default = telemetry.NewDir(dir)
		counter.Open()
		counter.OpenAndRotate() // should panic
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestOpenAPIMisuse(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)

	// Test that Open and OpenAndRotate cannot be used simultaneously.
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), telemetryDirEnvVar+"="+t.TempDir())

	if err := cmd.Run(); err == nil {
		t.Error("Failed to detect API misuse: no error from calling both Open and OpenAndRotate")
	}
}
