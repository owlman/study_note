// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.21

package countertest

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"golang.org/x/telemetry/counter"
	"golang.org/x/telemetry/internal/telemetry"
	"golang.org/x/telemetry/internal/testenv"
)

func TestReadCounter(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)
	c := counter.New("foobar")

	got, err := ReadCounter(c)
	if err != nil {
		t.Errorf("ReadCounter = (%d, %v), want (0,nil)", got, err)
	}
	if got != 0 {
		t.Fatalf("ReadCounter = %d, want 0", got)
	}

	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			c.Inc()
			wg.Done()
		}()
	}
	wg.Wait()
	if got, err := ReadCounter(c); err != nil || got != 100 {
		t.Errorf("ReadCounter = (%v, %v), want (%v, nil)", got, err, 100)
	}
}

func TestReadStackCounter(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)
	c := counter.NewStack("foobar", 8)

	if got, err := ReadStackCounter(c); err != nil || len(got) != 0 {
		t.Errorf("ReadStackCounter = (%q, %v), want (%v, nil)", got, err, 0)
	}

	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			c.Inc() // one stack!
			wg.Done()
		}()
	}
	wg.Wait()

	got, err := ReadStackCounter(c)
	if err != nil || len(got) != 1 {
		t.Fatalf("ReadStackCounter = (%v, %v), want to read one entry", stringify(got), err)
	}
	for k, v := range got {
		if !strings.Contains(k, t.Name()) || v != 100 {
			t.Fatalf("ReadStackCounter = %v, want a stack counter with value 100", got)
		}
	}
}

func TestSupport(t *testing.T) {
	if SupportedPlatform == telemetry.DisabledOnPlatform {
		t.Errorf("supported mismatch: us %v, telemetry.internal.Disabled %v",
			SupportedPlatform, telemetry.DisabledOnPlatform)
	}
}

func stringify(m map[string]uint64) string {
	kv := make([]string, 0, len(m))
	for k, v := range m {
		kv = append(kv, fmt.Sprintf("%q:%v", k, v))
	}
	slices.Sort(kv)
	return "{" + strings.Join(kv, " ") + "}"
}
