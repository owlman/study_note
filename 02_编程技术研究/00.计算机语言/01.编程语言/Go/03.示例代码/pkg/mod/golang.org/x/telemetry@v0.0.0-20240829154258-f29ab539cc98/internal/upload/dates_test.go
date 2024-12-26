// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/telemetry/counter"
	"golang.org/x/telemetry/internal/configtest"
	"golang.org/x/telemetry/internal/regtest"
	"golang.org/x/telemetry/internal/telemetry"
	"golang.org/x/telemetry/internal/testenv"
)

// CreateTestUploadServer creates a test server that records the uploaded data.
// The server is closed as part of cleaning up t.
func CreateTestUploadServer(t *testing.T) (*httptest.Server, func() [][]byte) {
	s := &uploadQueue{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("invalid request received: %v", err)
			http.Error(w, "read failed", http.StatusBadRequest)
			return
		}
		s.Append(buf)
	}))
	t.Cleanup(srv.Close)
	return srv, s.Get
}

type uploadQueue struct {
	mu   sync.Mutex
	data [][]byte
}

func (s *uploadQueue) Append(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = append(s.data, data)
}

func (s *uploadQueue) Get() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data
}

func CreateTestUploadConfig(t *testing.T, counterNames, stackCounterNames []string) *telemetry.UploadConfig {
	goVersion, progPath, progVersion := regtest.ProgramInfo(t)
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

func TestDates(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)

	prog := regtest.NewProgram(t, "prog", func() int {
		counter.Inc("testing")
		counter.NewStack("aStack", 4).Inc()
		return 0
	})

	// Run a fake program that produces a counter file in the telemetryDir.
	// readCountFileInfo will give us a template counter file content
	// based on the counter file.
	telemetryDir := t.TempDir()
	if out, err := regtest.RunProg(t, telemetryDir, prog); err != nil {
		t.Fatalf("failed to run program: %s", out)
	}
	cs := readCountFileInfo(t, filepath.Join(telemetryDir, "local"))
	uc := CreateTestUploadConfig(t, nil, []string{"aStack"})
	env := configtest.LocalProxyEnv(t, uc, "v1.2.3")

	const today = "2020-01-24"
	const yesterday = "2020-01-23"
	telemetryEnableTime, _ := time.Parse(dateFormat, "2019-12-01") // back-date the telemetry acceptance
	tests := []Test{                                               // each date must be different to subvert the parse cache
		{ // test that existing counters and ready files are not uploaded if they span data before telemetry was enabled
			name:   "beforefirstupload",
			today:  "2019-12-04",
			date:   "2019-12-03",
			begins: "2019-12-01",
			ends:   "2019-12-03",
			readys: []string{"2019-12-01", "2019-12-02"},
			// We get one local report: the newly created report.
			// It is not ready as it begins on the same day that telemetry was
			// enabled, and we err on the side of assuming it holds data from before
			// the user turned on uploading.
			wantLocal: 1,
			// The report for 2019-12-01 is still ready, because it was not uploaded.
			// This could happen in practice if the user disabled and then reenabled
			// telmetry.
			wantReady: 1,
			// The report for 2019-12-02 was uploaded.
			wantUploadeds: 1,
		},
		{ // test that existing counters and ready files are uploaded they only contain data after telemetry was enabled
			name:          "oktoupload",
			today:         "2019-12-10",
			date:          "2019-12-09",
			begins:        "2019-12-02",
			ends:          "2019-12-09",
			readys:        []string{"2019-12-07"},
			wantLocal:     1,
			wantUploadeds: 2, // Both new report and existing report are uploaded.
		},
		{ // test that an old countfile is removed and no reports generated
			name:   "oldcountfile",
			today:  today,
			date:   "2020-01-01",
			begins: "2020-01-01",
			ends:   olderThan(t, today, distantPast, "oldcountfile"),
			// one local; readys, uploads are empty, and there should be nothing left
			wantLocal: 1,
		},
		{ // test that a count file expiring today is left alone
			name:       "todayscountfile",
			today:      today,
			date:       "2020-01-02",
			begins:     "2020-01-08",
			ends:       today,
			wantCounts: 1,
		},
		{ // test that a count file expiring yesterday generates reports
			name:          "yesterdaycountfile",
			today:         today,
			date:          "2020-01-03",
			begins:        "2020-01-16",
			ends:          yesterday,
			wantLocal:     1,
			wantUploadeds: 1,
		},
		{ // count file already has local report, remove count file
			name:       "alreadydonelocal",
			today:      today,
			date:       "2020-01-04",
			begins:     "2020-01-16",
			ends:       yesterday,
			locals:     []string{yesterday},
			wantCounts: 0,
			wantLocal:  1,
		},
		{ // count file already has upload report, remove count file
			name:          "alreadydoneuploaded",
			today:         today,
			date:          "2020-01-05",
			begins:        "2020-01-16",
			ends:          "2020-01-23",
			uploads:       []string{"2020-01-23"},
			wantCounts:    0, // count file has been used, remove it
			wantLocal:     0, // no local report generated
			wantUploadeds: 1, // the existing uploaded report
		},
		{ // for some reason there's a ready file in the future, don't upload it
			name:       "futurereadyfile",
			today:      "2020-01-24",
			date:       "2020-01-06",
			begins:     "2020-01-16",
			ends:       "2020-01-24", // count file not expired
			readys:     []string{"2020-01-25"},
			wantCounts: 1, // active count file
			wantReady:  1, // existing premature ready file
		},
	}

	for _, tx := range tests {
		t.Run(tx.name, func(t *testing.T) {
			telemetryDir := t.TempDir()

			srv, uploaded := CreateTestUploadServer(t)

			dbg := filepath.Join(telemetryDir, "debug")
			if err := os.MkdirAll(dbg, 0777); err != nil {
				t.Fatal(err)
			}
			uploader, err := newUploader(RunConfig{
				TelemetryDir: telemetryDir,
				UploadURL:    srv.URL,
				Env:          env,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer uploader.Close()
			if err := uploader.dir.SetModeAsOf("on", telemetryEnableTime); err != nil {
				t.Fatal(err)
			}
			uploader.startTime = mustParseDate(tx.today)

			wantUploadCount := doTest(t, uploader, &tx, cs)
			if got := len(uploaded()); wantUploadCount != got {
				t.Errorf("server got %d upload requests, want %d", got, wantUploadCount)
			}
		})
	}
}

func mustParseDate(d string) time.Time {
	x, err := time.Parse(telemetry.DateOnly, d)
	if err != nil {
		log.Fatalf("couldn't parse time %s", d)
	}
	return x
}

// return a day more than 'old' before 'today'
func olderThan(t *testing.T, today string, old time.Duration, nm string) string {
	x, err := time.Parse(telemetry.DateOnly, today)
	if err != nil {
		t.Errorf("%q not a day in test %s (%v)", today, nm, err)
		return today // so test should fail
	}
	ans := x.Add(-old - 24*time.Hour)
	msg := ans.Format(telemetry.DateOnly)
	return msg
}

// Test is a single test.
//
// All dates are in YYYY-MM-DD format.
type Test struct {
	name  string // the test name; only used for descriptive output
	today string // the date of the fake upload
	// count file
	date         string // the date in of the upload file name; must be unique among tests
	begins, ends string // the begin and end date stored in the counter metadata

	// Dates of load reports in the local dir.
	locals []string

	// Dates of upload reports in the local dir.
	readys []string

	// Dates of reports already uploaded.
	uploads []string

	// number of expected results
	wantCounts    int
	wantReady     int
	wantLocal     int
	wantUploadeds int
}

// Information from the counter file so its contents can be
// modified for tests
type countFileInfo struct {
	beginOffset, endOffset int    // where the dates are in the file
	buf                    []byte // counter file contents
	namePrefix             string // the part of its name before the date
	originalName           string // its original name
}

// return useful information from the counter file to be used
// in creating tests. also compute and return the UploadConfig
// Note that there must be exactly one counter file in localDir.
func readCountFileInfo(t *testing.T, localDir string) *countFileInfo {
	fis, err := os.ReadDir(localDir)
	if err != nil {
		t.Fatal(err)
	}
	var countFileName string
	var countFileBuf []byte
	for _, f := range fis {
		if strings.HasSuffix(f.Name(), ".count") {
			countFileName = filepath.Join(localDir, f.Name())
			buf, err := os.ReadFile(countFileName)
			if err != nil {
				t.Fatal(err)
			}
			countFileBuf = buf
			break
		}
	}
	if len(countFileBuf) == 0 {
		t.Fatalf("no contents read for %s", countFileName)
	}

	var cfilename string = countFileName
	cfilename = filepath.Base(cfilename)
	flds := strings.Split(cfilename, "-")
	if len(flds) != 7 {
		t.Fatalf("got %d fields, expected 7 (%q)", len(flds), cfilename)
	}
	pr := strings.Join(flds[:4], "-") + "-"

	ans := countFileInfo{
		buf:          countFileBuf,
		namePrefix:   pr,
		originalName: countFileName,
	}
	idx := bytes.Index(countFileBuf, []byte("TimeEnd: "))
	if idx < 0 {
		t.Fatalf("couldn't find TimeEnd in count file %q", countFileBuf[:100])
	}
	ans.endOffset = idx + len("TimeEnd: ")
	idx = bytes.Index(countFileBuf, []byte("TimeBegin: "))
	if idx < 0 {
		t.Fatalf("couldn't find TimeBegin in countfile %q", countFileBuf[:100])
	}
	ans.beginOffset = idx + len("TimeBegin: ")
	return &ans
}

func doTest(t *testing.T, u *uploader, test *Test, known *countFileInfo) int {
	// set up directory contents
	contents := bytes.Join([][]byte{
		known.buf[:known.beginOffset],
		[]byte(test.begins),
		known.buf[known.beginOffset+len("YYYY-MM-DD") : known.endOffset],
		[]byte(test.ends),
		known.buf[known.endOffset+len("YYYY-MM-DD"):],
	}, nil)
	filename := known.namePrefix + test.date + ".v1.count"
	if err := os.MkdirAll(u.dir.LocalDir(), 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(u.dir.LocalDir(), filename), contents, 0666); err != nil {
		t.Fatalf("writing count file for %s (%s): %v", test.name, filename, err)
	}
	for _, x := range test.locals {
		nm := fmt.Sprintf("local.%s.json", x)
		if err := os.WriteFile(filepath.Join(u.dir.LocalDir(), nm), []byte{}, 0666); err != nil {
			t.Fatalf("%v writing local file %s", err, nm)
		}
	}
	for _, x := range test.readys {
		nm := fmt.Sprintf("%s.json", x)
		if err := os.WriteFile(filepath.Join(u.dir.LocalDir(), nm), []byte{}, 0666); err != nil {
			t.Fatalf("%v writing ready file %s", err, nm)
		}
	}
	if len(test.uploads) > 0 {
		os.MkdirAll(u.dir.UploadDir(), 0777)
	}
	for _, x := range test.uploads {
		nm := fmt.Sprintf("%s.json", x)
		if err := os.WriteFile(filepath.Join(u.dir.UploadDir(), nm), []byte{}, 0666); err != nil {
			t.Fatalf("%v writing upload %s", err, nm)
		}
	}

	// run
	u.Run()

	// check results
	var cfiles, rfiles, lfiles, ufiles int
	fis, err := os.ReadDir(u.dir.LocalDir())
	if err != nil {
		t.Errorf("%v reading localdir %s", err, u.dir.LocalDir())
		return 0
	}
	for _, f := range fis {
		switch {
		case strings.HasSuffix(f.Name(), ".v1.count"):
			cfiles++
		case f.Name() == "weekends": // ok
		case strings.HasPrefix(f.Name(), "local."):
			lfiles++
		case strings.HasSuffix(f.Name(), ".json"):
			rfiles++
		default:
			t.Errorf("for %s, unexpected local file %s", test.name, f.Name())
		}
	}

	var logcnt int
	logs, err := os.ReadDir(u.dir.DebugDir())
	if err == nil {
		logcnt = len(logs)
	}
	if logcnt != 1 {
		t.Errorf("expected 1 log file, got %d", logcnt)
	}

	fis, err = os.ReadDir(u.dir.UploadDir())
	if err != nil {
		t.Errorf("%v reading uploaddir %s", err, u.dir.UploadDir())
		return 0
	}
	ufiles = len(fis) // assume there's nothing but .json reports
	if test.wantCounts != cfiles {
		t.Errorf("%s: got %d countfiles, wanted %d", test.name, cfiles, test.wantCounts)
	}
	if test.wantReady != rfiles {
		t.Errorf("%s: got %d ready files, wanted %d", test.name, rfiles, test.wantReady)
	}
	if test.wantLocal != lfiles {
		t.Errorf("%s: got %d localfiles, wanted %d", test.name, lfiles, test.wantLocal)
	}
	if test.wantUploadeds != ufiles {
		t.Errorf("%s: got %d uploaded files, wanted %d", test.name, ufiles, test.wantUploadeds)
	}
	return ufiles - len(test.uploads)
}
