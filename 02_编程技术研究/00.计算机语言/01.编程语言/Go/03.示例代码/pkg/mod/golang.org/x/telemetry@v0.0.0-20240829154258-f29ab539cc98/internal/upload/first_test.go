// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// make sure we can talk to the test server
// In practice this test runs last, so is somewhat superfluous,
// but it also checks that uploads and reads from the channel are matched
func TestSimpleServer(t *testing.T) {
	srv, uploaded := CreateTestUploadServer(t)

	url := srv.URL
	resp, err := http.Post(url, "text/plain", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("%#v", resp.StatusCode)
	}
	got := uploaded()
	want := [][]byte{[]byte("hello")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

// make sure computeRandom() gets enough small values
// in case SamplingRate is as small as .001
func TestRandom(t *testing.T) {
	// This test, being statistical, is intrinsically flaky
	// It has failed once in its first 3 months at 4.5,
	// so change the criterion to 7 sigma.
	const N = 102400 // 35msec on an M1 mac
	cnt := 0
	for i := 0; i < N; i++ {
		if computeRandom() < 1.0/1024 {
			cnt++
		}
	}
	// cnt has a binomial distribution. The normal
	// approximation has mu=N*p=100, sigma=sqrt(N*p*(1-p))=10
	// We reject if cnt is off by 45, which happens about 1/300,000
	// if the computeRandom() is truly uniform. That is, the
	// test will be flaky about 3 times in a million.
	if cnt < 30 || cnt > 170 {
		t.Errorf("cnt %d more than 7 sigma(10) from mean(100)", cnt)
	}
}
