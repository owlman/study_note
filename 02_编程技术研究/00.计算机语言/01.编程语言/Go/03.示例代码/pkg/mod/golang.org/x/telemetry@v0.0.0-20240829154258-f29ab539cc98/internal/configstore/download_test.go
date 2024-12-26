// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package configstore_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/telemetry/internal/configstore"
	"golang.org/x/telemetry/internal/configtest"
	"golang.org/x/telemetry/internal/telemetry"
	"golang.org/x/telemetry/internal/testenv"
)

func TestDownload(t *testing.T) {
	testenv.NeedsGo(t)

	configVersion := "v0.1.0"
	in := &telemetry.UploadConfig{
		GOOS:      []string{"darwin"},
		GOARCH:    []string{"amd64", "arm64"},
		GoVersion: []string{"1.20.3", "1.20.4"},
		Programs: []*telemetry.ProgramConfig{{
			Name:     "gopls",
			Versions: []string{"v0.11.0"},
			Counters: []telemetry.CounterConfig{{
				Name: "foobar",
				Rate: 2,
			}},
		}},
	}

	env := configtest.LocalProxyEnv(t, in, configVersion)
	testCases := []struct {
		version string
		want    telemetry.UploadConfig
	}{
		{version: configVersion, want: *in},
		{version: "latest", want: *in},
	}
	for _, tc := range testCases {
		t.Run(tc.version, func(t *testing.T) {
			got, _, err := configstore.Download(tc.version, env)
			if err != nil {
				t.Fatal("failed to download:", err)
			}

			want := tc.want
			if !reflect.DeepEqual(*got, want) {
				t.Errorf("Download(latest, _) = %v\nwant %v", stringify(got), stringify(want))
			}
		})
	}

	t.Run("invalidversion", func(t *testing.T) {
		got, ver, err := configstore.Download("nonexisting", env)
		if err == nil {
			t.Fatalf("download succeeded unexpectedly: %v %+v", ver, got)
		}
		if !strings.Contains(err.Error(), "invalid version") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func stringify(x any) string {
	ret, err := json.MarshalIndent(x, "", " ")
	if err != nil {
		return fmt.Sprintf("json.Marshal failed - %v", err)
	}
	return string(ret)
}
