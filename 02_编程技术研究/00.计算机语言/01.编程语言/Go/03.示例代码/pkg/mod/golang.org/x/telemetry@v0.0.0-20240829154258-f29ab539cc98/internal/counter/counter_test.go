// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package counter

// Builders at
// https://build.golang.org/?repo=golang.org%2fx%2ftelemetry

// there are troubles with tests in Windows. all open files have to
// be closed by the test so the test directory can be removed.
// Once defaultFile is closed, no more tests can be run as
// Open() will fault. (This is mysterious.)

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/telemetry/internal/telemetry"
	"golang.org/x/telemetry/internal/testenv"
)

func TestBasic(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)

	t.Logf("GOOS %s GOARCH %s", runtime.GOOS, runtime.GOARCH)
	setup(t)
	var f file
	defer close(&f)
	c := f.New("gophers")
	c.Add(9)
	f.rotate()
	if f.err != nil {
		t.Fatal(f.err)
	}
	current := f.current.Load()
	if current == nil {
		t.Fatal("no mapped file")
	}
	c.Add(0x90)

	name := current.f.Name()
	t.Logf("wrote %s:\n%s", name, hexDump(current.mapping.Data))

	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	pf, err := Parse(name, data)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]uint64{"gophers": 0x99}
	if !reflect.DeepEqual(pf.Count, want) {
		t.Errorf("pf.Count = %v, want %v", pf.Count, want)
	}
}

func TestMissingLocalDir(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)
	err := os.RemoveAll(telemetry.Default.LocalDir())
	if err != nil {
		t.Fatal(err)
	}
	TestBasic(t)
}

func TestParallel(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)

	t.Logf("GOOS %s GOARCH %s", runtime.GOOS, runtime.GOARCH)
	setup(t)
	var f file
	defer close(&f)

	c := f.New("manygophers")

	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			c.Inc()
			wg.Done()
		}()
	}
	wg.Wait()
	f.rotate()
	if f.err != nil {
		t.Fatal(f.err)
	}
	current := f.current.Load()
	if current == nil {
		t.Fatal("no mapped file")
	}
	name := current.f.Name()
	t.Logf("wrote %s:\n%s", name, hexDump(current.mapping.Data))

	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	pf, err := Parse(name, data)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]uint64{"manygophers": 100}
	if !reflect.DeepEqual(pf.Count, want) {
		t.Errorf("pf.Count = %v, want %v", pf.Count, want)
	}
}

// close ensures that the given mapped file is closed. On Windows, this is
// necessary prior to test cleanup.
// TODO(rfindley): rename.
func close(f *file) {
	mf := f.current.Load()
	if mf == nil {
		// telemetry might have been off
		return
	}
	mf.close()
}

func TestLarge(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)
	t.Logf("GOOS %s GOARCH %s", runtime.GOOS, runtime.GOARCH)
	setup(t)

	var f file
	defer close(&f)
	f.rotate()
	for i := int64(0); i < 10000; i++ {
		c := f.New(fmt.Sprint("gophers", i))
		c.Add(i*i + 1)
	}
	for i := int64(0); i < 10000; i++ {
		c := f.New(fmt.Sprint("gophers", i))
		c.Add(i / 2)
	}
	current := f.current.Load()
	if current == nil {
		t.Fatal("no mapped file")
	}
	name := current.f.Name()

	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	pf, err := Parse(name, data)
	if err != nil {
		t.Fatal(err)
	}

	var errcnt int
	for i := uint64(0); i < 10000; i++ {
		key := fmt.Sprint("gophers", i)
		want := 1 + i*i + i/2
		if n := pf.Count[key]; n != want {
			// print out the first few errors
			t.Errorf("Count[%s] = %d, want %d", key, n, want)
			errcnt++
			if errcnt > 5 {
				return
			}
		}
	}
}

func TestCorruption_Truncation(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)

	if runtime.GOOS == "windows" {
		t.Skip("windows does not permit truncating a file that is mapped")
	}

	defer func(crash bool) {
		CrashOnBugs = crash
	}(CrashOnBugs)
	CrashOnBugs = false // we're intentionally introducing corruption below

	// In golang/go#68311, it appeared that telemetry became stuck in an infinite
	// loop of re-mapping as a result of a corrupt counter file.
	//
	// While the specific conditions that led to corruption are not understood,
	// the infinite loop was reproducible by truncating the counter file after
	// extension.

	setup(t)
	var f file
	defer close(&f)
	f.rotate1()

	// Populate enough data to extend the file beyond its minimum length.
	const numCounters = 1000
	for i := int64(0); i < numCounters; i++ {
		f.New(fmt.Sprint("gophers", i)).Inc()
	}

	current := f.current.Load()
	if current == nil {
		t.Fatal("no mapped file")
	}
	if err := current.f.Truncate(minFileLen); err != nil {
		t.Fatalf("truncating %q: %v", current.f.Name(), err)
	}

	// Increment the same counters that were created above. This should exercise
	// the corruption, as counter heads will point to file locations that no
	// longer exist.
	var f2 file
	defer close(&f2)
	f2.rotate1()
	for i := int64(0); i < numCounters; i++ {
		f2.New(fmt.Sprint("gophers", i)).Inc()
	}
}

func TestRepeatedNew(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)

	t.Logf("GOOS %s GOARCH %s", runtime.GOOS, runtime.GOARCH)
	setup(t)
	var f file
	defer close(&f)
	f.rotate()
	f.New("gophers")
	c1ptr := f.lookup("gophers")
	f.New("gophers")
	c2ptr := f.lookup("gophers")
	if c1ptr != c2ptr {
		t.Errorf("c1ptr = %p, c2ptr = %p, want same", c1ptr, c2ptr)
	}
}

func hexDump(data []byte) string {
	lines := strings.SplitAfter(hex.Dump(data), "\n")
	var keep []string
	for len(lines) > 0 {
		line := lines[0]
		keep = append(keep, line)
		lines = lines[1:]
		const allZeros = "00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00"
		if strings.Contains(line, allZeros) {
			i := 0
			for i < len(lines) && strings.Contains(lines[i], allZeros) {
				i++
			}
			if i > 2 {
				keep = append(keep, "*\n", lines[i-1])
				lines = lines[i:]
			}
		}
	}
	return strings.Join(keep, "")
}

func TestNewFile(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)

	t.Logf("GOOS %s GOARCH %s", runtime.GOOS, runtime.GOARCH)
	setup(t)

	now := CounterTime().UTC()
	year, month, day := now.Date()
	// preserve time location as done in (*file).filename.
	testStartTime := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)

	// test that completely new files have dates well in the future
	// Try 20 times to get 20 different random numbers.
	for i := 0; i < 20; i++ {
		var f file
		c := f.New("gophers")
		// shouldn't see a file yet
		fi, err := os.ReadDir(telemetry.Default.LocalDir())
		if err != nil {
			t.Fatal(err)
		}
		if len(fi) != 0 {
			t.Fatalf("len(fi) = %d, want 0", len(fi))
		}
		c.Add(9)
		// still shouldn't see a file
		fi, err = os.ReadDir(telemetry.Default.LocalDir())
		if err != nil {
			close(&f)
			t.Fatal(err)
		}
		if len(fi) != 0 {
			close(&f)
			t.Fatalf("len(fi) = %d, want 0", len(fi))
		}
		f.rotate()
		// now we should see a count file and a weekends file
		fi, _ = os.ReadDir(telemetry.Default.LocalDir())
		if len(fi) != 2 {
			close(&f)
			t.Fatalf("len(fi) = %d, want 2", len(fi))
		}
		var countFile, weekendsFile string
		for _, f := range fi {
			switch f.Name() {
			case "weekends":
				weekendsFile = f.Name()
				// while we're here, check that is ok
				buf, err := os.ReadFile(filepath.Join(telemetry.Default.LocalDir(), weekendsFile))
				if err != nil {
					t.Fatal(err)
				}
				buf = bytes.TrimSpace(buf)
				if len(buf) == 0 || buf[0] < '0' || buf[0] >= '7' {
					t.Errorf("weekends file has bad data: %q", buf)
				}
			default:
				countFile = f.Name()
			}
		}

		buf, err := os.ReadFile(filepath.Join(telemetry.Default.LocalDir(), countFile))
		if err != nil {
			close(&f)
			t.Fatal(err)
		}
		cf, err := Parse(countFile, buf)
		if err != nil {
			close(&f)
			t.Fatal(err)
		}
		timeEnd, err := time.Parse(time.RFC3339, cf.Meta["TimeEnd"])
		if err != nil {
			close(&f)
			t.Fatal(err)
		}
		days := (timeEnd.Sub(testStartTime)) / (24 * time.Hour)
		if days <= 0 || days > 7 {
			timeBegin, _ := time.Parse(time.RFC3339, cf.Meta["TimeBegin"])
			t.Logf("testStartTime: %v file: %v TimeBegin: %v TimeEnd: %v", testStartTime, fi[0].Name(), timeBegin, timeEnd)
			t.Errorf("%d: days = %d, want 7 < days <= 14", i, days)
		}
		close(&f)
		// remove the file for the next iteration of the loop
		os.Remove(filepath.Join(telemetry.Default.LocalDir(), countFile))
		os.Remove(filepath.Join(telemetry.Default.LocalDir(), weekendsFile))
	}
}

func TestWeekends(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)

	setup(t)
	// get all the 49 combinations of today and when the week ends
	for i := 0; i < 7; i++ {
		CounterTime = future(i)
		for index := range "0123456" {
			os.WriteFile(filepath.Join(telemetry.Default.LocalDir(), "weekends"), []byte{byte(index + '0')}, 0666)
			var f file
			c := f.New("gophers")
			c.Add(7)
			f.rotate1()
			fis, err := os.ReadDir(telemetry.Default.LocalDir())
			if err != nil {
				t.Fatal(err)
			}
			weekends := time.Weekday(-1)
			var begins, ends time.Time
			for _, fi := range fis {
				// ignore errors for brevity: something else will fail
				if fi.Name() == "weekends" {
					buf, _ := os.ReadFile(filepath.Join(telemetry.Default.LocalDir(), fi.Name()))
					buf = bytes.TrimSpace(buf)
					weekends = time.Weekday(buf[0] - '0')
				} else if strings.HasSuffix(fi.Name(), ".count") {
					buf, _ := os.ReadFile(filepath.Join(telemetry.Default.LocalDir(), fi.Name()))
					parsed, _ := Parse(fi.Name(), buf)
					begins, _ = time.Parse(time.RFC3339, parsed.Meta["TimeBegin"])
					ends, _ = time.Parse(time.RFC3339, parsed.Meta["TimeEnd"])
				}
			}
			if weekends < 0 {
				for _, f := range fis {
					t.Errorf("in %s, weekends is %d", f.Name(), weekends)
				}
				continue
			}
			delta := int(ends.Sub(begins) / (24 * time.Hour))
			// if we're an old user, we should have a <=7 day report
			// if we're a new user, we should have a <=7+7 day report
			more := 0
			if delta <= 0+more || delta > 7+more {
				t.Errorf("delta %d, expected %d<delta<=%d",
					delta, more, more+7)
			}
			if weekends != ends.Weekday() {
				t.Errorf("weekends %s unexpecteledy not end day %s", weekends, ends.Weekday())
			}
			// On Windows, we must unmap f.current before removing files below.
			close(&f)

			// remove files for the next iteration of the loop
			for _, f := range fis {
				os.Remove(filepath.Join(telemetry.Default.LocalDir(), f.Name()))
			}
		}
	}
}

func future(days int) func() time.Time {
	return func() time.Time {
		return time.Now().UTC().AddDate(0, 0, days)
	}
}

func TestStack(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)
	t.Logf("GOOS %s GOARCH %s", runtime.GOOS, runtime.GOARCH)
	setup(t)
	var f file
	defer close(&f)
	f.rotate()

	c := f.NewStack("foo", 5)
	c.Inc()
	c.Inc()
	names := c.Names()
	if len(names) != 2 {
		t.Fatalf("got %d names, want 2", len(names))
	}
	// each name should be 4 lines, and the two names should
	// differ only in the second line.
	n0 := strings.Split(names[0], "\n")
	n1 := strings.Split(names[1], "\n")
	if len(n0) != 4 || len(n1) != 4 {
		t.Errorf("got %d and %d lines, want 4 (%q,%q)", len(n0), len(n1), n0, n1)
	}
	for i := 0; i < 4 && i < len(n0) && i < len(n1); i++ {
		if i == 1 {
			continue
		}
		if n0[i] != n1[i] {
			t.Errorf("line %d differs:\n%s\n%s", i, n0[i], n1[i])
		}
	}
	// check that ReadStack gives the same results
	mp, err := ReadStack(c)
	if len(mp) != 2 {
		t.Errorf("ReadStack returned %d values, expected 2", len(mp))
	}
	for k, v := range mp {
		if v != 1 {
			t.Errorf("got %d for %q, expected 1", v, k)
		}
	}

	oldnames := make(map[string]bool)
	for _, nm := range names {
		oldnames[nm] = true
	}
	for i := 0; i < 2; i++ {
		fn(t, 4, c)
	}
	newnames := make(map[string]bool)
	for _, nm := range c.Names() {
		if !oldnames[nm] {
			newnames[nm] = true
		}
	}
	// expect 5 new names, one for each level of recursion
	if len(newnames) != 5 {
		t.Errorf("got %d new names, want 5", len(newnames))
	}
	// make sure the new names contain compression
	for k := range newnames {
		if !strings.Contains(k, "\"") {
			t.Errorf("new name %q does not contain \"", k)
		}
	}
	// look inside. old names should have a count of 1, new ones 2
	for _, ct := range c.Counters() {
		if ct == nil {
			t.Fatal("nil counter")
		}
		_, err := Read(ct)
		if err != nil {
			t.Errorf("failed to read known counter %v", err)
		}
		if ct.ptr.count == nil {
			t.Errorf("%q has nil ptr.count", ct.Name())
			continue
		}
		if oldnames[ct.Name()] && ct.ptr.count.Load() != 1 {
			t.Errorf("old name %q has count %d, want 1", ct.Name(), ct.ptr.count.Load())
		}
		if newnames[ct.Name()] && ct.ptr.count.Load() != 2 {
			t.Errorf("new name %q has count %d, want 2", ct.Name(), ct.ptr.count.Load())
		}
	}
	// check that Parse expands compressed counter names
	current := f.current.Load()
	if current == nil {
		t.Fatal("no mapped file")
	}
	data := current.mapping.Data
	fname := "2023-01-01.v1.count" // bogus file name required by Parse.
	theFile, err := Parse(fname, data)
	if err != nil {
		t.Fatal(err)
	}
	// We know what lines should appear in the stack counter names,
	// although line numbers outside our control might change.
	// A less fragile test would just check that " doesn't appear
	known := map[string]bool{
		"foo": true,
		"golang.org/x/telemetry/internal/counter.fn":        true,
		"golang.org/x/telemetry/internal/counter.TestStack": true,
		"runtime.goexit":  true,
		"testing.tRunner": true,
	}
	counts := theFile.Count
	for k := range counts {
		ll := strings.Split(k, "\n")
		for _, line := range ll {
			ix := strings.LastIndex(line, ":")
			if ix < 0 {
				continue // foo, for instance
			}
			line = line[:ix]
			if !known[line] {
				t.Errorf("unexpected line %q", line)
			}
		}
	}
}

// fn calls itself n times recursively while incrementing the stack counter.
func fn(t *testing.T, n int, c *StackCounter) {
	c.Inc()
	if n > 0 {
		fn(t, n-1, c)
	}
}

func setup(t *testing.T) {
	log.SetFlags(log.Lshortfile)
	telemetry.Default = telemetry.NewDir(t.TempDir()) // new dir for each test
	os.MkdirAll(telemetry.Default.LocalDir(), 0777)
	os.MkdirAll(telemetry.Default.UploadDir(), 0777)
	t.Cleanup(func() {
		CounterTime = func() time.Time { return time.Now().UTC() }
	})
}

func (f *file) New(name string) *Counter {
	return &Counter{name: name, file: f}
}

func (f *file) NewStack(name string, depth int) *StackCounter {
	return &StackCounter{name: name, depth: depth, file: f}
}
