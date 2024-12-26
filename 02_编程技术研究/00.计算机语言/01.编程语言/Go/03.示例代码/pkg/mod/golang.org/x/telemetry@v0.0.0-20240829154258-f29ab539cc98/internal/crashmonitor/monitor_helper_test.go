// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crashmonitor

// This file opens back doors for testing.

var (
	WriteSentinel        = writeSentinel
	TelemetryCounterName = telemetryCounterName
)

func SetIncrementCounter(f func(name string)) {
	incrementCounter = f
}

func SetChildExitHook(f func()) {
	childExitHook = f
}
