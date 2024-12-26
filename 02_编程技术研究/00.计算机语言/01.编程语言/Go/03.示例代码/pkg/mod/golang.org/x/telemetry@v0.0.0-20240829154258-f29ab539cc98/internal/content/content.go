// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package content

import (
	"embed"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

//go:embed *
var FS embed.FS

//go:generate go run generate.go

// RunESBuild runs esbuild for all content directories.
// If watch is set, RunESBuild instructs esbuild to watch the content
// directories, and runs esbuild in a separate goroutine.
func RunESBuild(watch bool) {
	_, file, _, _ := runtime.Caller(0)
	curDir := filepath.Dir(file)
	cmdDir := filepath.Join(curDir, "..", "..", "godev", "devtools", "cmd", "esbuild")
	for _, dir := range []string{"gotelemetryview", "shared", "telemetrygodev"} {
		d := filepath.Join(curDir, dir)
		args := []string{"run", ".", "--outdir", filepath.Join(d, "static")}
		if watch {
			args = append(args, "--watch", d)
		}
		args = append(args, d)
		cmd := exec.Command("go", args...)
		cmd.Dir = cmdDir
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		}
		if watch {
			go func() {
				if err := cmd.Wait(); err != nil {
					log.Fatal(err)
				}
			}()
		} else {
			if err := cmd.Wait(); err != nil {
				log.Fatal(err)
			}
		}
	}
}
