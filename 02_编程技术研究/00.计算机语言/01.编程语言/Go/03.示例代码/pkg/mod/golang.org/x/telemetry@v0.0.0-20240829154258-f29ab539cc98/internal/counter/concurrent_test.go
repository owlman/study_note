// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package counter_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/telemetry/counter/countertest"
	"golang.org/x/telemetry/internal/counter"
	"golang.org/x/telemetry/internal/regtest"
	"golang.org/x/telemetry/internal/telemetry"
	"golang.org/x/telemetry/internal/testenv"
)

func init() {
	// Catch any bugs encountered while mapping counters.
	counter.CrashOnBugs = true
}

func TestConcurrentExtension(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)

	// This test verifies that files may be concurrently extended: when one file
	// discovers that its entries exceed the mapped data, it remaps the data.

	// Both programs populate enough new records to extend the file multiple
	// times.
	const numCounters = 50000
	prog1 := regtest.NewProgram(t, "inc1", func() int {
		for i := 0; i < numCounters; i++ {
			counter.New(fmt.Sprint("gophers", i)).Inc()
		}
		return 0
	})
	prog2 := regtest.NewProgram(t, "inc2", func() int {
		for i := numCounters; i < 2*numCounters; i++ {
			counter.New(fmt.Sprint("gophers", i)).Inc()
		}
		return 0
	})

	dir := t.TempDir()
	now := time.Now().UTC()

	// Run a no-op program in the telemetry dir to ensure that the weekends file
	// exists, and avoid the race described in golang/go#68390.
	// (We could also call countertest.Open here, but better to avoid mutating
	// state in the current process for a test that is otherwise hermetic)
	prog0 := regtest.NewProgram(t, "init", func() int { return 0 })
	if _, err := regtest.RunProgAsOf(t, dir, now, prog0); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Run the programs concurrently.
	go func() {
		defer wg.Done()
		if out, err := regtest.RunProgAsOf(t, dir, now, prog1); err != nil {
			t.Errorf("prog1 failed: %v; output:\n%s", err, out)
		}
	}()
	go func() {
		defer wg.Done()
		if out, err := regtest.RunProgAsOf(t, dir, now, prog2); err != nil {
			t.Errorf("prog2 failed: %v; output:\n%s", err, out)
		}
	}()

	wg.Wait()

	counts := readCountsForDir(t, telemetry.NewDir(dir).LocalDir())
	if got, want := len(counts), 2*numCounters; got != want {
		t.Errorf("Got %d counters, want %d", got, want)
	}

	for name, value := range counts {
		if value != 1 {
			t.Errorf("count(%s) = %d, want 1", name, value)
		}
	}
}

func readCountsForDir(t *testing.T, dir string) map[string]uint64 {
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var countFiles []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".count") {
			countFiles = append(countFiles, filepath.Join(dir, entry.Name()))
		}
	}
	if len(countFiles) != 1 {
		t.Fatalf("found %d count files, want 1; directory contents: %v", len(countFiles), entries)
	}

	counters, _, err := countertest.ReadFile(countFiles[0])
	if err != nil {
		t.Fatal(err)
	}
	return counters
}
