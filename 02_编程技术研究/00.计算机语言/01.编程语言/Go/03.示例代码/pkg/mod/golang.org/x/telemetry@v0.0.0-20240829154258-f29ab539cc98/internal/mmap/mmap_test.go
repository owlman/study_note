// Copyright 2024 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mmap_test

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"

	"golang.org/x/telemetry/internal/counter"
	"golang.org/x/telemetry/internal/mmap"
	"golang.org/x/telemetry/internal/testenv"
)

// If the sharedFileEnv environment variable is set,
// increment an atomic value in that file rather than
// run the test.
const sharedFileEnv = "MMAP_TEST_SHARED_FILE"

func TestMain(m *testing.M) {
	if name := os.Getenv(sharedFileEnv); name != "" {
		_, mapping, err := openMapped(name)
		if err != nil {
			log.Fatalf("openMapped failed: %v", err)
		}

		v := (*atomic.Uint64)(unsafe.Pointer(&mapping.Data[0]))
		v.Add(1)
		// Exit without explicitly calling munmap/close.
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func openMapped(name string) (*os.File, *mmap.Data, error) {
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, nil, fmt.Errorf("open failed: %v", err)
	}
	data, err := mmap.Mmap(f)
	if err != nil {
		return nil, nil, fmt.Errorf("Mmap failed: %v", err)
	}
	return f, data, nil
}

// Via golang/go#68389 and golang/go#68458, we learned that 64-bit atomics were
// unreliable on linux/arm in Go 1.21. This was fixed in
// https://go.dev/cl/525637, but only for ARMv7 and later.
func skipIfLinuxArm(t *testing.T) {
	if runtime.GOOS == "linux" && runtime.GOARCH == "arm" {
		t.Skipf("64-bit atomics may not work on linux/arm")
	}
}

func TestSharedMemory(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)
	skipIfLinuxArm(t)

	// This test verifies that Mmap'ed files are usable for concurrent
	// cross-process atomic operations.

	dir := t.TempDir()
	name := filepath.Join(dir, "shared.count")

	var zero [8]byte
	if err := os.WriteFile(name, zero[:], 0666); err != nil {
		t.Fatal(err)
	}

	// Fork+exec the current test process.
	// Child processes atomically increment the counter file in shared memory.

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	const concurrency = 100
	var wg sync.WaitGroup
	env := append(os.Environ(), sharedFileEnv+"="+name)
	for i := 0; i < concurrency; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(exe)
			cmd.Env = env

			if err := cmd.Run(); err != nil {
				t.Errorf("subcommand #%d failed: %v", i, err)
			}
		}()
	}

	wg.Wait()

	data, err := counter.ReadMapped(name)
	if err != nil {
		t.Fatalf("final read failed: %v", err)
	}
	v := (*atomic.Uint64)(unsafe.Pointer(&data[0]))
	if got := v.Load(); got != concurrency {
		t.Errorf("incremented %d times, want %d", got, concurrency)
	}
}

func TestMultipleMaps(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)
	skipIfLinuxArm(t)

	// This test verifies that multiple views of an mmapp'ed file may
	// simultaneously exist for the current process. This is relied upon by
	// counter concurrency logic.

	dir := t.TempDir()
	name := filepath.Join(dir, "shared.count")

	var zero [8]byte
	if err := os.WriteFile(name, zero[:], 0666); err != nil {
		t.Fatal(err)
	}

	var (
		mappings []*mmap.Data
		values   []*atomic.Uint64 // mapped counts
	)

	const nMaps = 3
	for i := 0; i < nMaps; i++ {
		f, mapping, err := openMapped(name)
		if err != nil {
			t.Fatal(err)
		}
		mappings = append(mappings, mapping)
		i := i
		defer func() {
			if i > 0 {
				mmap.Munmap(mapping)
			}
			f.Close()
		}()
		values = append(values, (*atomic.Uint64)(unsafe.Pointer(&mapping.Data[0])))
	}

	var wg sync.WaitGroup
	const nAdds = 100
	for _, v := range values {
		v := v
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				v.Add(1)
			}
		}()
	}
	wg.Wait()
	for i, v := range values {
		if got, want := v.Load(), uint64(nMaps*nAdds); got != want {
			t.Errorf("counter %d has value %d, want %d", i, got, want)
		}
	}
	mmap.Munmap(mappings[0]) // other mappings should remain valid
	for i, v := range values[1:] {
		if got, want := v.Load(), uint64(nMaps*nAdds); got != want {
			t.Errorf("counter %d has value %d, want %d", i, got, want)
		}
	}
}
