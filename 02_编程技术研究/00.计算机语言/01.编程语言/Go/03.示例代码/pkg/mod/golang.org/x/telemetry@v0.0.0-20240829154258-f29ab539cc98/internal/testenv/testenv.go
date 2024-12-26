// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testenv contains helper functions for skipping tests
// based on which tools are present in the environment.
package testenv

import (
	"bytes"
	"fmt"
	"go/build"
	"os/exec"
	"runtime"
	"sync"
	"testing"

	"golang.org/x/telemetry/internal/telemetry"
)

// NeedsLocalhostNet skips t if networking does not work for ports opened
// with "localhost".
func NeedsLocalhostNet(t testing.TB) {
	switch runtime.GOOS {
	case "js", "wasip1":
		t.Skipf(`Listening on "localhost" fails on %s; see https://go.dev/issue/59718`, runtime.GOOS)
	}
}

var (
	hasGoOnce sync.Once
	hasGoErr  error
)

// HasGo checks whether the current system has 'go'.
func HasGo() error {
	hasGoOnce.Do(func() {
		cmd := exec.Command("go", "env", "GOROOT")
		out, err := cmd.Output()
		if err != nil { // cannot run go.
			hasGoErr = fmt.Errorf("%v: %v", cmd, err)
			return
		}
		out = bytes.TrimSpace(out)
		if len(out) == 0 { // unusual, incomplete go installation.
			hasGoErr = fmt.Errorf("%v: no GOROOT - incomplete go installation", cmd)
		}
	})
	return hasGoErr
}

// NeedsGo skips t if the current system does not have 'go'  or
// cannot run them with exec.Command.
func NeedsGo(t testing.TB) {
	if err := HasGo(); err != nil {
		t.Skipf("skipping test: go is not available - %v", err)
	}
}

// SkipIfUnsupportedPlatform skips test if the current os/arch
// are not support.
func SkipIfUnsupportedPlatform(t testing.TB) {
	t.Helper()
	if telemetry.DisabledOnPlatform {
		t.Skip("telemetry is unsupported on this platform")
	}
}

// MustHaveExec checks that the current system can start new processes
// using os.StartProcess or (more commonly) exec.Command.
// If not, MustHaveExec calls t.Skip with an explanation.
func MustHaveExec(t testing.TB) {
	switch runtime.GOOS {
	case "wasip1", "js", "ios":
		t.Skipf("skipping test: may not be able to exec subprocess on %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

// Go1Point returns the x in Go 1.x.
func Go1Point() int {
	for i := len(build.Default.ReleaseTags) - 1; i >= 0; i-- {
		var version int
		if _, err := fmt.Sscanf(build.Default.ReleaseTags[i], "go1.%d", &version); err != nil {
			continue
		}
		return version
	}
	panic("bad release tags")
}

// NeedsGo1Point skips t if the Go version used to run the test is older than
// 1.x.
func NeedsGo1Point(t testing.TB, x int) {
	if Go1Point() < x {
		t.Helper()
		t.Skipf("running Go version %q is version 1.%d, older than required 1.%d", runtime.Version(), Go1Point(), x)
	}
}
