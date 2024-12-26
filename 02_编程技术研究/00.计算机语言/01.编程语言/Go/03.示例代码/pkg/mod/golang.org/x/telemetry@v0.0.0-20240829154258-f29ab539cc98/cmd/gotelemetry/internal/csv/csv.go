// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// csv dumps all the active counters. The output is
// a sequence of lines
// value,"counter-name",program, version,go-version,goos, garch
// sorted by counter name. It looks at the files in
// telemetry.LocalDir that are counter files or local reports
// By design it pays no attention to dates. The combination
// of program version and go version are deemed sufficient.
package csv

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/telemetry/internal/counter"
	"golang.org/x/telemetry/internal/telemetry"
)

type file struct {
	path, name string
	// one of counters or report is set
	counters *counter.File
	report   *telemetry.Report
}

func Csv() {
	files, err := readdir(telemetry.Default.LocalDir(), nil)
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f.name, "v1.count") {
			buf, err := os.ReadFile(f.path)
			if err != nil {
				log.Print(err)
				continue
			}
			cf, err := counter.Parse(f.name, buf)
			if err != nil {
				log.Print(err)
				continue
			}
			f.counters = cf
		} else if strings.HasSuffix(f.name, ".json") {
			buf, err := os.ReadFile(f.path)
			if err != nil {
				log.Print(err)
				continue
			}
			var x telemetry.Report
			if err := json.Unmarshal(buf, &x); err != nil {
				log.Print(err)
				continue
			}
			f.report = &x
		}
	}
	printTable(files)
}

type record struct {
	goos, garch, program, version, goversion string
	cntr                                     string
	count                                    int
}

func printTable(files []*file) {
	lines := make(map[string]*record)
	work := func(k string, v int64, rec *record) {
		x, ok := lines[k]
		if !ok {
			x = new(record)
			*x = *rec
			x.cntr = k
		}
		x.count += int(v)
		lines[k] = x
	}
	worku := func(k string, v uint64, rec *record) {
		work(k, int64(v), rec)
	}
	for _, f := range files {
		if f.counters != nil {
			var rec record
			rec.goos = f.counters.Meta["GOOS"]
			rec.garch = f.counters.Meta["GOARCH"]
			rec.program = f.counters.Meta["Program"]
			rec.version = f.counters.Meta["Version"]
			rec.goversion = f.counters.Meta["GoVersion"]
			for k, v := range f.counters.Count {
				worku(k, v, &rec)
			}
		} else if f.report != nil {
			for _, p := range f.report.Programs {
				var rec record
				rec.goos = p.GOOS
				rec.garch = p.GOARCH
				rec.goversion = p.GoVersion
				rec.program = p.Program
				rec.version = p.Version
				for k, v := range p.Counters {
					work(k, v, &rec)
				}
				for k, v := range p.Stacks {
					work(k, v, &rec)
				}
			}
		}
	}
	keys := make([]string, 0, len(lines))
	for k := range lines {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		printRecord(lines[k])
	}
}

func printRecord(r *record) {
	fmt.Printf("%d,%q,%s,%s,%s,%s,%s\n", r.count, r.cntr, r.program,
		r.version, r.goversion, r.goos, r.garch)
}

func readdir(dir string, files []*file) ([]*file, error) {
	fi, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, f := range fi {
		files = append(files, &file{path: filepath.Join(dir, f.Name()), name: f.Name()})
	}
	return files, nil
}
