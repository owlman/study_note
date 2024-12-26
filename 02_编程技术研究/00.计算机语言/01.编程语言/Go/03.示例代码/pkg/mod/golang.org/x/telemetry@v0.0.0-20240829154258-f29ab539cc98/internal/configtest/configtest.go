// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package configtest provides a helper for testing using a local proxy
// containing a fake upload config.
package configtest

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/telemetry/internal/configstore"
	"golang.org/x/telemetry/internal/proxy"
	"golang.org/x/telemetry/internal/telemetry"
)

// LocalProxyEnv writes a proxy directory for the given upload config, and
// returns a go environment to use for fetching that config from a local
// file-based proxy.
//
// This environment should be passed to [configstore.Download].
func LocalProxyEnv(t *testing.T, cfg *telemetry.UploadConfig, version string) []string {
	t.Helper()

	dir := t.TempDir()

	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshaling config failed: %v", err)
	}
	dirPath := fmt.Sprintf("%v@%v/", configstore.ModulePath, version)
	files := map[string][]byte{
		dirPath + "go.mod":      []byte("module " + configstore.ModulePath + "\n\ngo 1.20\n"),
		dirPath + "config.json": encoded,
	}
	proxyURI, err := proxy.WriteProxy(filepath.Join(dir, "proxy"), files)
	if err != nil {
		t.Fatalf("writing proxy failed: %v", err)
	}

	env := []string{
		"GOPROXY=" + proxyURI, // Use the fake proxy.
		"GONOSUMDB=*",         // Skip verifying checksum against sum.golang.org.
		"GOMODCACHE=" + filepath.Join(dir, "modcache"), // Don't pollute system module cache.
	}
	t.Cleanup(func() {
		cmd := exec.Command("go", "clean", "-modcache")
		cmd.Env = append(cmd.Environ(), env...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("go clean -modcache failed: %v\n%s", err, out)
		}
	})
	return env
}
