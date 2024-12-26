// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/telemetry/counter"
	"golang.org/x/telemetry/internal/configstore"
	"golang.org/x/telemetry/internal/configtest"
	"golang.org/x/telemetry/internal/regtest"
	"golang.org/x/telemetry/internal/telemetry"
	"golang.org/x/telemetry/internal/testenv"
	"golang.org/x/telemetry/internal/upload"
)

// runConfig sets up an upload environment for the provided test, with a
// fake proxy allowing the given counters, and a fake upload server.
//
// The returned RunConfig is ready to pass to Run to upload the given
// directory. The second return is a function to fetch all uploaded reports.
//
// For convenience, runConfig also sets the mode in telemetryDir to "on",
// back-dated to a time in the past. Callers that want to run the upload with a
// different mode can reset as necessary.
//
// All associated resources are cleaned up with t.Clean.
func runConfig(t *testing.T, telemetryDir string, counters, stackCounters []string) (upload.RunConfig, func() [][]byte) {
	t.Helper()

	if err := telemetry.NewDir(telemetryDir).SetModeAsOf("on", time.Now().Add(-365*24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	srv, uploaded := upload.CreateTestUploadServer(t)
	uc := upload.CreateTestUploadConfig(t, counters, stackCounters)
	env := configtest.LocalProxyEnv(t, uc, "v1.2.3")

	return upload.RunConfig{
		TelemetryDir: telemetryDir,
		UploadURL:    srv.URL,
		LogWriter:    testWriter{"", t},
		Env:          env,
	}, uploaded
}

// testWriter is an io.Writer wrapping t.Log.
type testWriter struct {
	prefix string
	t      *testing.T
}

func (w testWriter) Write(p []byte) (n int, err error) {
	w.t.Log(w.prefix + strings.TrimSuffix(string(p), "\n")) // trim newlines added by logging
	return len(p), nil
}

func TestRun_Basic(t *testing.T) {
	// Check the correctness of a single upload to the local server.

	testenv.SkipIfUnsupportedPlatform(t)

	prog := regtest.NewProgram(t, "prog", func() int {
		counter.Inc("knownCounter")
		counter.Inc("unknownCounter")
		counter.NewStack("aStack", 4).Inc()
		return 0
	})

	// produce a counter file (timestamped with "today")
	telemetryDir := t.TempDir()
	if out, err := regtest.RunProgAsOf(t, telemetryDir, time.Now().Add(-8*24*time.Hour), prog); err != nil {
		t.Fatalf("failed to run program: %s", out)
	}

	// Running the program should produce a counter file.
	checkTelemetryFiles(t, telemetryDir, telemetryFiles{counterFiles: 1})

	// Aside: writing the "debug" file here reproduces a scenario observed in the
	// past where the "debug" directory could not be read.
	// (there is no issue to reference for additional context, unfortunately)
	logName := filepath.Join(telemetryDir, "debug")
	err := os.WriteFile(logName, nil, 0666) // must be done before calling Run
	if err != nil {
		t.Fatal(err)
	}

	// Run the upload.
	cfg, getUploads := runConfig(t, telemetryDir, []string{"knownCounter", "aStack"}, nil)
	if err := upload.Run(cfg); err != nil {
		t.Fatal(err)
	}

	// The upload process should have deleted the counter file, and produced both
	// a local and uploaded report.
	checkTelemetryFiles(t, telemetryDir, telemetryFiles{localReports: 1, uploadedReports: 1})

	// Check that the uploaded report matches our expectations exactly.
	uploads := getUploads()
	if len(uploads) != 1 {
		t.Fatalf("got %d uploads, want 1", len(uploads))
	}
	var got telemetry.Report
	if err := json.Unmarshal(uploads[0], &got); err != nil {
		t.Fatal(err)
	}
	if got.Week == "" {
		t.Errorf("Uploaded report missing Week field:\n%s", uploads[0])
	}
	if len(got.Programs) != 1 {
		t.Fatalf("got %d uploaded programs, want 1", len(got.Programs))
	}
	gotProgram := got.Programs[0]
	want := telemetry.Report{
		Week: got.Week, // volatile
		X:    got.X,    // volatile
		Programs: []*telemetry.ProgramReport{{
			Program:   "upload.test",
			Version:   "",
			GoVersion: gotProgram.GoVersion, // easiest to read this from the report
			GOOS:      runtime.GOOS,
			GOARCH:    runtime.GOARCH,
			Counters: map[string]int64{
				"knownCounter": 1,
			},
			Stacks: map[string]int64{},
		}},
		Config: "v1.2.3",
	}
	gotFormatted, err := json.MarshalIndent(got, "", "\t")
	if err != nil {
		t.Fatal(err)
	}
	wantFormatted, err := json.MarshalIndent(want, "", "\t")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(gotFormatted), string(wantFormatted); got != want {
		t.Errorf("Mismatching uploaded report:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

type telemetryFiles struct {
	counterFiles      int
	localReports      int
	unuploadedReports int
	uploadedReports   int
	// Other files like mode or weekends are intentionally omitted, because they
	// are less interesting internal details.
}

// checkTelemetryFiles checks that the state of telemetryDir matches the
// desired telemetryFiles.
func checkTelemetryFiles(t *testing.T, telemetryDir string, want telemetryFiles) {
	t.Helper()

	dir := telemetry.NewDir(telemetryDir)

	countFiles := func(dir, pattern string) int {
		count := 0
		fis, err := os.ReadDir(dir)
		if err != nil {
			return 0 // missing directory
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, fi := range fis {
			if re.MatchString(fi.Name()) {
				count++
			}
		}
		return count
	}
	got := telemetryFiles{
		counterFiles:      countFiles(dir.LocalDir(), `\.v1\.count`),
		localReports:      countFiles(dir.LocalDir(), `^local\..*\.json$`),
		unuploadedReports: countFiles(dir.LocalDir(), `^[0-9].*\.json$`),
		uploadedReports:   countFiles(dir.UploadDir(), `^[0-9].*\.json$`),
	}
	if got != want {
		t.Errorf("got telemetry files %+v, want %+v", got, want)
	}
}

func TestRun_Retries(t *testing.T) {
	// Check that the Run handles upload server status codes appropriately,
	// and that retries behave as expected.

	testenv.SkipIfUnsupportedPlatform(t)

	prog := regtest.NewIncProgram(t, "prog", "counter")

	tests := []struct {
		initialStatus   int
		initialFiles    telemetryFiles
		filesAfterRetry telemetryFiles
	}{
		{
			http.StatusOK,
			telemetryFiles{localReports: 1, uploadedReports: 1},
			telemetryFiles{localReports: 1, uploadedReports: 1},
		},
		{
			http.StatusBadRequest,
			telemetryFiles{localReports: 1},
			telemetryFiles{localReports: 1},
		},
		{
			http.StatusInternalServerError,
			telemetryFiles{localReports: 1, unuploadedReports: 1},
			telemetryFiles{localReports: 1, uploadedReports: 1},
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprint(test.initialStatus), func(t *testing.T) {
			telemetryDir := t.TempDir()
			if out, err := regtest.RunProgAsOf(t, telemetryDir, time.Now().Add(-8*24*time.Hour), prog); err != nil {
				t.Fatalf("failed to run program: %s", out)
			}

			// Start an upload server that returns the given status code.
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.initialStatus)
			}))
			t.Cleanup(srv.Close)

			// Enable uploads.
			dir := telemetry.NewDir(telemetryDir)
			if err := dir.SetModeAsOf("on", time.Now().Add(-365*24*time.Hour)); err != nil {
				t.Fatal(err)
			}

			// Write the proxy.
			uc := upload.CreateTestUploadConfig(t, []string{"counter"}, nil)
			env := configtest.LocalProxyEnv(t, uc, "v1.2.3")

			// Run the upload.
			badCfg := upload.RunConfig{
				TelemetryDir: telemetryDir,
				UploadURL:    srv.URL,
				Env:          env,
			}
			if err := upload.Run(badCfg); err != nil {
				t.Fatal(err)
			}

			// Check that the upload left the telemetry directory in the desired state.
			checkTelemetryFiles(t, telemetryDir, test.initialFiles)

			// Now re-run the upload with a succeeding upload server.
			goodCfg, _ := runConfig(t, telemetryDir, []string{"counter"}, nil)
			if err := upload.Run(goodCfg); err != nil {
				t.Fatal(err)
			}

			// Check files after retrying.
			checkTelemetryFiles(t, telemetryDir, test.filesAfterRetry)
		})
	}
}

func TestRun_MultipleUploads(t *testing.T) {
	// This test checks that [upload.Run] produces multiple reports when counters
	// span more than a week.

	testenv.SkipIfUnsupportedPlatform(t)

	// This program is run at two different dates.
	prog := regtest.NewIncProgram(t, "prog", "counter1")

	// Create two counter files to upload, at least a week apart.
	telemetryDir := t.TempDir()
	asof1 := time.Now().Add(-15 * 24 * time.Hour)
	if out, err := regtest.RunProgAsOf(t, telemetryDir, asof1, prog); err != nil {
		t.Fatalf("failed to run program: %s", out)
	}
	asof2 := time.Now().Add(-8 * 24 * time.Hour)
	if out, err := regtest.RunProgAsOf(t, telemetryDir, asof2, prog); err != nil {
		t.Fatalf("failed to run program: %s", out)
	}

	cfg, getUploads := runConfig(t, telemetryDir, []string{"counter1", "counter2"}, nil)
	if err := upload.Run(cfg); err != nil {
		t.Fatal(err)
	}

	uploads := getUploads()
	if got, want := len(uploads), 2; got != want {
		t.Fatalf("got %d uploads, want %d", got, want)
	}
	for _, upload := range uploads {
		report := string(upload)
		if !strings.Contains(report, "counter1") {
			t.Errorf("Didn't get an upload for counter1. Report:\n%s", report)
		}
	}
}

func TestRun_EmptyUpload(t *testing.T) {
	// This test verifies that an empty counter file does not cause uploads of
	// another week's reports to fail.

	testenv.SkipIfUnsupportedPlatform(t)

	// prog1 runs in week 1, and increments no counter.
	prog1 := regtest.NewIncProgram(t, "prog1")
	// prog2 runs in week 2.
	prog2 := regtest.NewIncProgram(t, "prog2", "week2")

	telemetryDir := t.TempDir()

	// Create two counter files to upload, at least a week apart.
	// Week 1 has no counters, which in the past caused the both uploads to fail.
	asof1 := time.Now().Add(-15 * 24 * time.Hour)
	if out, err := regtest.RunProgAsOf(t, telemetryDir, asof1, prog1); err != nil {
		t.Fatalf("failed to run program: %s", out)
	}
	asof2 := time.Now().Add(-8 * 24 * time.Hour)
	if out, err := regtest.RunProgAsOf(t, telemetryDir, asof2, prog2); err != nil {
		t.Fatalf("failed to run program: %s", out)
	}

	cfg, getUploads := runConfig(t, telemetryDir, []string{"week1", "week2"}, nil)
	if err := upload.Run(cfg); err != nil {
		t.Fatal(err)
	}

	// Check that we got one upload, for week 2.
	uploads := getUploads()
	if got, want := len(uploads), 1; got != want {
		t.Fatalf("got %d uploads, want %d", got, want)
	}
	for _, upload := range uploads {
		report := string(upload)
		if !strings.Contains(report, "week2") {
			t.Errorf("Didn't get an upload for week2. Report:\n%s", report)
		}
	}
}

func TestRun_MissingDate(t *testing.T) {
	// This test verifies that a counter file with corrupt metadata does not
	// prevent the uploader from uploading another week's reports.

	testenv.SkipIfUnsupportedPlatform(t)

	prog := regtest.NewIncProgram(t, "prog", "counter")

	telemetryDir := t.TempDir()

	// Create two counter files to upload, a week apart.
	asof1 := time.Now().Add(-15 * 24 * time.Hour)
	if out, err := regtest.RunProgAsOf(t, telemetryDir, asof1, prog); err != nil {
		t.Fatalf("failed to run program: %s", out)
	}

	// Corrupt the week 1 counter file.
	{
		localDir := telemetry.NewDir(telemetryDir).LocalDir()
		fis, err := os.ReadDir(localDir)
		if err != nil {
			t.Fatal(err)
		}
		var countFiles []string
		for _, fi := range fis {
			if strings.HasSuffix(fi.Name(), ".v1.count") {
				countFiles = append(countFiles, filepath.Join(localDir, fi.Name()))
			}
		}
		if len(countFiles) != 1 {
			t.Fatalf("after first RunProgAsOf, found %d count files, want 1", len(countFiles))
		}
		countFile := countFiles[0]
		data, err := os.ReadFile(countFile)
		if err != nil {
			t.Fatal(err)
		}
		// Importantly, the byte replacement here has the same length.
		// If not, the entire file (and not just metadata) would be corrupt, due to
		// the header length mismatch.
		corrupted := bytes.Replace(data, []byte(`TimeBegin:`), []byte(`TimxBegin:`), 1)
		if err := os.WriteFile(countFile, corrupted, 0666); err != nil {
			t.Fatal(err)
		}
	}

	asof2 := time.Now().Add(-8 * 24 * time.Hour)
	if out, err := regtest.RunProgAsOf(t, telemetryDir, asof2, prog); err != nil {
		t.Fatalf("failed to run program: %s", out)
	}

	cfg, getUploads := runConfig(t, telemetryDir, []string{"counter"}, nil)
	if err := upload.Run(cfg); err != nil {
		t.Fatal(err)
	}

	// Check that we got one upload, for week 2.
	uploads := getUploads()
	if got, want := len(uploads), 1; got != want {
		t.Fatalf("got %d uploads, want %d", got, want)
	}
	report := string(uploads[0])
	if !strings.Contains(report, "counter") {
		t.Errorf("Didn't get an upload for counter. Report:\n%s", report)
	}
}

func TestRun_ModeHandling(t *testing.T) {
	// This test verifies that the uploader honors the telemetry mode, as well as
	// its asof date.

	testenv.SkipIfUnsupportedPlatform(t)

	prog := regtest.NewIncProgram(t, "prog1", "counter")

	tests := []struct {
		mode                string
		wantConfigDownloads int64
		wantUploads         int
	}{
		{"off", 0, 0},
		{"local", 0, 0},
		{"on", 1, 1}, // only the second week is uploadable
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			telemetryDir := t.TempDir()
			// Create two counter files to upload, at least a week apart.
			now := time.Now()
			asof1 := now.Add(-15 * 24 * time.Hour)
			if out, err := regtest.RunProgAsOf(t, telemetryDir, asof1, prog); err != nil {
				t.Fatalf("failed to run program: %s", out)
			}
			asof2 := now.Add(-8 * 24 * time.Hour)
			if out, err := regtest.RunProgAsOf(t, telemetryDir, asof2, prog); err != nil {
				t.Fatalf("failed to run program: %s", out)
			}

			cfg, getUploads := runConfig(t, telemetryDir, []string{"counter"}, nil)

			// Enable telemetry as of 10 days ago. This should prevent the first week
			// from being uploaded, but not the second.
			if err := telemetry.NewDir(telemetryDir).SetModeAsOf(test.mode, now.Add(-10*24*time.Hour)); err != nil {
				t.Fatal(err)
			}

			downloadsBefore := configstore.Downloads()
			if err := upload.Run(cfg); err != nil {
				t.Fatal(err)
			}

			if got := configstore.Downloads() - downloadsBefore; got != test.wantConfigDownloads {
				t.Errorf("configstore.Download called: %v, want %v", got, test.wantConfigDownloads)
			}

			uploads := getUploads()
			if gotUploads := len(uploads); gotUploads != test.wantUploads {
				t.Fatalf("got %d uploads, want %d", gotUploads, test.wantUploads)
			}
		})
	}
}

func TestRun_DebugLog(t *testing.T) {
	// This test verifies that the uploader honors the telemetry mode, as well as
	// its asof date.

	testenv.SkipIfUnsupportedPlatform(t)

	prog := regtest.NewIncProgram(t, "prog1", "counter")

	tests := []struct {
		name          string
		setup         func(t *testing.T) (telemetryDir string, err error)
		wantDebugLogs int
		wantUploads   int
	}{
		{
			name: "valid",
			setup: func(t *testing.T) (string, error) {
				userConfigDir := "user config" // test use of space in the name
				if runtime.GOOS == "windows" {
					userConfigDir = "userconfig" // windows doesn't allow space in dir name
				}
				telemetryDir := filepath.Join(t.TempDir(), userConfigDir)
				return telemetryDir, os.MkdirAll(filepath.Join(telemetryDir, "debug"), 0755)
			},
			wantDebugLogs: 1,
			wantUploads:   1,
		},
		{
			name: "nodebug",
			setup: func(t *testing.T) (string, error) {
				return t.TempDir(), nil
			},
			wantUploads: 1,
		},
		{
			name: "not a directory", // debug log setup error shouldn't prevent uploading.
			setup: func(t *testing.T) (string, error) {
				telemetryDir := t.TempDir()
				return telemetryDir, os.WriteFile(filepath.Join(telemetryDir, "debug"), nil, 0666)
			},
			wantUploads: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			telemetryDir, err := test.setup(t)
			if err != nil {
				t.Fatalf("failed to configure the telemetry and debug directories: %v", err)
			}
			now := time.Now()
			asof := now.Add(-8 * 24 * time.Hour)
			if out, err := regtest.RunProgAsOf(t, telemetryDir, asof, prog); err != nil {
				t.Fatalf("failed to run program: %s", out)
			}

			cfg, getUploads := runConfig(t, telemetryDir, []string{"counter"}, nil)
			if err := upload.Run(cfg); err != nil {
				t.Fatal(err)
			}

			uploads := getUploads()
			if gotUploads := len(uploads); gotUploads != test.wantUploads {
				t.Errorf("got %d uploads, want %d", gotUploads, test.wantUploads)
			}
			debugLogs := getDebugLogs(t, filepath.Join(telemetryDir, "debug"))
			if gotDebugLogs := len(debugLogs); gotDebugLogs != test.wantDebugLogs {
				t.Fatalf("got %d debug logs, want %d", gotDebugLogs, test.wantDebugLogs)
			}
		})
	}
}

func TestRun_Concurrent(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)

	prog := regtest.NewIncProgram(t, "prog1", "counter")

	telemetryDir := t.TempDir()
	now := time.Now().UTC()

	// Seed two weeks of uploads.
	// These should *all* be uploaded as they will be neither too old,
	// nor too new.
	incCount := 0
	for i := -21; i < -7; i++ {
		incCount++
		asof := now.Add(time.Duration(i) * 24 * time.Hour)
		if out, err := regtest.RunProgAsOf(t, telemetryDir, asof, prog); err != nil {
			t.Fatalf("failed to run program: %s", out)
		}
	}

	cfg, getUploads := runConfig(t, telemetryDir, []string{"counter"}, nil)
	cfg.StartTime = now // avoid date skew with counter time

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		i := i
		wg.Add(1)
		cfg2 := cfg
		cfg2.LogWriter = testWriter{fmt.Sprintf("uploader #%d: ", i), t} // use a unique log prefix for this uploader
		go func() {
			defer wg.Done()
			if err := upload.Run(cfg2); err != nil {
				t.Errorf("upload.Run #%d failed: %v", i, err)
			}
		}()
	}
	wg.Wait()

	uploads := getUploads()
	uploadedCount := 0
	for i, upload := range uploads {
		var got telemetry.Report
		if err := json.Unmarshal(upload, &got); err != nil {
			t.Fatalf("error unmarshalling uploaded report: %v\ncontents:%s", err, upload)
		}
		if got, want := len(got.Programs), 1; got != want {
			t.Fatalf("got %d programs in upload #%d, want %d", got, i, want)
		}
		uploadedCount += int(got.Programs[0].Counters["counter"])
	}
	if uploadedCount != incCount {
		t.Errorf("uploaded %d total observations, want %d", uploadedCount, incCount)
	}
}

func getDebugLogs(t *testing.T, debugDir string) []string {
	t.Helper()
	if stat, err := os.Stat(debugDir); err != nil || !stat.IsDir() {
		return nil
	}
	files, err := os.ReadDir(debugDir)
	if err != nil {
		return nil
	}
	var ret []string
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".log") {
			t.Logf("Ignoring %v", f.Name())
			continue
		}
		contents, err := os.ReadFile(filepath.Join(debugDir, f.Name()))
		if err != nil || !bytes.Contains(contents, []byte("mode on")) {
			t.Logf("Ignoring %v - unreadable or unexpected contents (err: %v)", f.Name(), err)
			continue
		}
		ret = append(ret, f.Name())
	}
	return ret
}
