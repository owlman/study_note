// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package regtest provides helpers for end-to-end testing
// involving counter and upload packages.
package regtest

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"golang.org/x/telemetry/counter"
	"golang.org/x/telemetry/counter/countertest"
	internalcounter "golang.org/x/telemetry/internal/counter"
	"golang.org/x/telemetry/internal/telemetry"
)

const (
	telemetryDirEnvVar = "_COUNTERTEST_RUN_TELEMETRY_DIR"
	asofEnvVar         = "_COUNTERTEST_ASOF"
	entryPointEnvVar   = "_COUNTERTEST_ENTRYPOINT"
)

var (
	telemetryDirEnvVarValue = os.Getenv(telemetryDirEnvVar)
	asofEnvVarValue         = os.Getenv(asofEnvVar)
	entryPointEnvVarValue   = os.Getenv(entryPointEnvVar)
)

// Program is a value that can be used to identify a program in the test.
type Program string

// NewProgram returns a Program value that can be used to identify a program
// to run by RunProg. The program must be registered with NewProgram before
// the first call to RunProg in the test function.
//
// RunProg runs this binary in a separate process with special environment
// variables that specify the entry point. When this binary runs with the
// environment variables that match the specified name, NewProgram calls
// the given fn and exits with the return value. Note that all the code
// before NewProgram is executed in both the main process and the subprocess.
func NewProgram(t *testing.T, name string, fn func() int) Program {
	if telemetryDirEnvVarValue != "" && entryPointEnvVarValue == name {
		// We are running the separate process that was spawned by RunProg.
		fmt.Fprintf(os.Stderr, "running program %q\n", name)
		if asofEnvVarValue != "" {
			asof, err := time.Parse(telemetry.DateOnly, asofEnvVarValue)
			if err != nil {
				log.Fatalf("error parsing asof time %q: %v", asof, err)
			}
			fmt.Fprintf(os.Stderr, "setting counter time to %s\n", name)
			internalcounter.CounterTime = func() time.Time {
				return asof
			}
		}
		countertest.Open(telemetryDirEnvVarValue)
		os.Exit(fn())
	}

	testName, _, _ := strings.Cut(t.Name(), "/")
	registered, ok := registeredPrograms[testName]
	if !ok {
		registered = make(map[string]bool)
	}
	if registered[name] {
		t.Fatalf("program %q was already registered", name)
	}
	registered[name] = true
	return Program(name)
}

// NewIncProgram returns a basic program that increments the given counters and
// exits with status 0.
func NewIncProgram(t *testing.T, name string, counters ...string) Program {
	return NewProgram(t, name, func() int {
		for _, c := range counters {
			counter.Inc(c)
		}
		return 0
	})
}

// registeredPrograms stores all registered program names to detect duplicate registrations.
var registeredPrograms = make(map[string]map[string]bool) // test name -> program name -> exist

// RunProg runs the program prog in a separate process with the specified
// telemetry directory. RunProg can be called multiple times in the same test,
// but all the programs must be registered with NewProgram before the first
// call to RunProg.
func RunProg(t *testing.T, telemetryDir string, prog Program) ([]byte, error) {
	return RunProgAsOf(t, telemetryDir, time.Time{}, prog)
}

// RunProgAsOf is like RunProg, but executes the program as of a specific
// counter time.
func RunProgAsOf(t *testing.T, telemetryDir string, asof time.Time, prog Program) ([]byte, error) {
	if telemetryDirEnvVarValue != "" {
		fmt.Fprintf(os.Stderr, "unknown program %q\n %s %s", prog, telemetryDirEnvVarValue, entryPointEnvVarValue)
		os.Exit(2)
	}
	testName, _, _ := strings.Cut(t.Name(), "/")
	testBin, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot determine the current process's executable name: %v", err)
	}

	// Spawn a subprocess to run the 'prog' by setting telemetryDirEnvVar.
	cmd := exec.Command(testBin, "-test.run", fmt.Sprintf("^%s$", testName))
	cmd.Env = append(os.Environ(), telemetryDirEnvVar+"="+telemetryDir, entryPointEnvVar+"="+string(prog))
	if !asof.IsZero() {
		cmd.Env = append(cmd.Env, asofEnvVar+"="+asof.Format(telemetry.DateOnly))
	}
	return cmd.CombinedOutput()
}

// ProgramInfo returns the go version, program name and version info the
// process would record in its counter file.
func ProgramInfo(t *testing.T) (goVersion, progPath, progVersion string) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("cannot read build info - it's likely this setup is unsupported by the counter package")
	}
	return telemetry.ProgramInfo(info)
}

// CreateTestUploadConfig creates a new upload config for the current program,
// permitting the given counters.
func CreateTestUploadConfig(t *testing.T, counterNames, stackCounterNames []string) *telemetry.UploadConfig {
	goVersion, progPath, progVersion := ProgramInfo(t)
	GOOS, GOARCH := runtime.GOOS, runtime.GOARCH
	programConfig := &telemetry.ProgramConfig{
		Name:     progPath,
		Versions: []string{progVersion},
	}
	for _, c := range counterNames {
		programConfig.Counters = append(programConfig.Counters, telemetry.CounterConfig{Name: c, Rate: 1})
	}
	for _, c := range stackCounterNames {
		programConfig.Stacks = append(programConfig.Stacks, telemetry.CounterConfig{Name: c, Rate: 1, Depth: 16})
	}
	return &telemetry.UploadConfig{
		GOOS:       []string{GOOS},
		GOARCH:     []string{GOARCH},
		SampleRate: 1.0,
		GoVersion:  []string{goVersion},
		Programs:   []*telemetry.ProgramConfig{programConfig},
	}
}
