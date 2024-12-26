// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package telemetry_test

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/telemetry"
	"golang.org/x/telemetry/counter"
	"golang.org/x/telemetry/counter/countertest"
	"golang.org/x/telemetry/internal/configtest"
	ic "golang.org/x/telemetry/internal/counter"
	"golang.org/x/telemetry/internal/regtest"
	it "golang.org/x/telemetry/internal/telemetry"
	"golang.org/x/telemetry/internal/testenv"
)

// These environment variables are used to coordinate the fork+exec subprocess
// started by TestStart.
const (
	runStartEnv     = "X_TELEMETRY_TEST_START"
	telemetryDirEnv = "X_TELEMETRY_TEST_START_TELEMETRY_DIR"
	uploadURLEnv    = "X_TELEMETRY_TEST_START_UPLOAD_URL"
	asofEnv         = "X_TELEMETRY_TEST_START_ASOF"
)

func TestMain(m *testing.M) {
	// TestStart can't use internal/regtest, because Start itself also uses
	// fork+exec to start a subprocess, which does not interact well with the
	// fork+exec trick used by regtest.RunProg.
	if prog := os.Getenv(runStartEnv); prog != "" {
		os.Exit(runProg(prog))
	}
	os.Exit(m.Run())
}

// runProg runs the given program.
// See the switch statement below.
func runProg(prog string) int {

	mustGetEnv := func(envvar string) string {
		v := os.Getenv(envvar)
		if v == "" {
			log.Fatalf("missing required environment var %q", envvar)
		}
		return v
	}

	// Get the fake time used by all programs.
	asof, err := time.Parse(it.DateOnly, mustGetEnv(asofEnv))
	if err != nil {
		log.Fatalf("parsing %s: %v", asofEnv, err)
	}

	// Set global state.
	ic.CounterTime = func() time.Time { return asof } // must be done before Open
	countertest.Open(mustGetEnv(telemetryDirEnv))

	switch prog {
	case "setmode":
		// Use the modified time above for the asof time.
		if err := it.Default.SetModeAsOf("on", asof); err != nil {
			log.Fatalf("setting mode: %v", err)
		}
	case "inc":
		// (CounterTime is already set above)
		counter.Inc("teststart/counter")

	case "start":
		res := telemetry.Start(telemetry.Config{
			// No need to set TelemetryDir since the Default dir is already set by countertest.Open.
			Upload:          true,
			UploadURL:       mustGetEnv(uploadURLEnv),
			UploadStartTime: asof,
		})
		res.Wait()
	default:
		log.Fatalf("unknown program %q", prog)
	}
	return 0
}

func execProg(t *testing.T, telemetryDir, prog string, asof time.Time, env ...string) {
	// Run the runStart function below, via a fork+exec trick.
	exe, err := os.Executable()
	if err != nil {
		t.Error(err)
		return
	}
	cmd := exec.Command(exe, "** TestStart **") // this unused arg is just for ps(1)
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, asofEnv+"="+asof.Format(it.DateOnly))
	cmd.Env = append(cmd.Env, telemetryDirEnv+"="+telemetryDir)
	cmd.Env = append(cmd.Env, runStartEnv+"="+prog) // see TestMain
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.Output()
	if err != nil {
		t.Errorf("program failed unexpectedly (%v)\n%s", err, out)
	}
}

func TestStart(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)
	testenv.MustHaveExec(t)

	// This test sets up a telemetry environment, and then runs a test program
	// that increments a counter, and uploads via telemetry.Start.

	telemetryDir := t.TempDir()

	uploaded := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploaded = true
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("error reading body: %v", err)
		} else {
			if body := string(body); !strings.Contains(body, "teststart/counter") {
				t.Errorf("upload does not contain \"teststart/counter\":\n%s", body)
			}
		}
	}))
	uploadEnv := []string{uploadURLEnv + "=" + server.URL}

	uc := regtest.CreateTestUploadConfig(t, []string{"teststart/counter"}, nil)
	uploadEnv = append(uploadEnv, configtest.LocalProxyEnv(t, uc, "v1.2.3")...)

	// Script programs.
	now := time.Now()
	execProg(t, telemetryDir, "setmode", now.Add(-30*24*time.Hour)) // back-date telemetry acceptance
	execProg(t, telemetryDir, "inc", now.Add(-8*24*time.Hour))      // increment the counter
	execProg(t, telemetryDir, "start", now, uploadEnv...)           // run start

	if !uploaded {
		t.Fatalf("no upload occurred on %v", os.Getpid())
	}
}

func TestConcurrentStart(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)
	testenv.MustHaveExec(t)

	telemetryDir := t.TempDir()

	var uploadMu sync.Mutex
	uploads := map[string]int{} // date -> uploads
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path
		if idx := strings.LastIndex(r.URL.Path, "/"); idx >= 0 {
			key = r.URL.Path[idx+len("/"):]
		}
		uploadMu.Lock()
		uploads[key]++
		uploadMu.Unlock()
	}))
	uploadEnv := []string{uploadURLEnv + "=" + server.URL}

	uc := regtest.CreateTestUploadConfig(t, []string{"teststart/counter"}, nil)
	uploadEnv = append(uploadEnv, configtest.LocalProxyEnv(t, uc, "v1.2.3")...)

	now := time.Now()
	execProg(t, telemetryDir, "setmode", now.Add(-365*24*time.Hour)) // back-date telemetry acceptance
	execProg(t, telemetryDir, "inc", now.Add(-8*24*time.Hour))       // increment the counter

	// Populate three weeks of counters to upload.
	for i := -28; i < -7; i++ { // Populate three weeks of counters to upload.
		execProg(t, telemetryDir, "inc", now.Add(time.Duration(i)*24*time.Hour))
	}

	// Run start concurrently.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			execProg(t, telemetryDir, "start", now, uploadEnv...)
		}()
	}
	wg.Wait()

	// Expect exactly three weeks to be uploaded.
	if got, want := len(uploads), 3; got != want {
		t.Errorf("got %d report dates, want %d", got, want)
	}
	for asof, n := range uploads {
		if n != 1 {
			t.Errorf("got %d reports for %s, want 1", n, asof)
		}
	}
}
