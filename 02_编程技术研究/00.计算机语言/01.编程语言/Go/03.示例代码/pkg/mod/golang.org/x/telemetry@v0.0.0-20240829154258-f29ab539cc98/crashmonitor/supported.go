// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crashmonitor

import ic "golang.org/x/telemetry/internal/crashmonitor"

// Supported reports whether the runtime supports [runtime.SetCrashOutput].
//
// TODO(adonovan): eliminate once go1.23+ is assured.
func Supported() bool { return ic.Supported() }
