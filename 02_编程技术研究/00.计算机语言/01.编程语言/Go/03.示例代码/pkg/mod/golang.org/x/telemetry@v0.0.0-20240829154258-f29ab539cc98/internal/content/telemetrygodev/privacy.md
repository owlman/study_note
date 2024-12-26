---
Title: Go Telemetry Privacy Policy
Layout: privacy.html
---

# Privacy Policy

_Last updated: January 24, 2024_

Go Telemetry is a way for Go toolchain programs to collect data about their
performance and usage. This data can help developers improve the language and
tools.

## What Go Telemetry Records {#collection}

Go toolchain programs, such as the `go` command and `gopls`, record certain information
about their own execution. This data is stored in local files on your computer,
specifically in the [`os.UserConfigDir()/go/telemetry/local`](https://pkg.go.dev/os#UserConfigDir) directory.

Here is what these files contain:

* Event counters: Information about how Go toolchain programs
are used.
* Stack traces: Details about program execution for troubleshooting.
* Basic system information: Your operating system, CPU architecture, and name and version of the Go tool being executed.

Importantly, these files do not contain personal or other
identifying information about you or your system.

## Data Privacy {#data-privacy}

By default, the data collected by Go Telemetry is kept only locally on your computer.

It is not shared with anyone unless you explicitly decide to enable Go Telemetry.
You can do this by running the command [`gotelemetry on`](#command) or using a command
in your integrated development environment (IDE).

Once enabled, Go Telemetry may decide once a week to upload reports to a Google
server.  A local copy of the uploaded reports is kept in the
[`os.UserConfigDir()/go/telemetry/remote`](https://pkg.go.dev/os#UserConfigDir) directory on the user's machine.
These reports include only approved counters and are collected in
accordance with the Google Privacy Policy, which you can find
at [Google Privacy Policy](https://policies.google.com/privacy).

The uploaded reports are also made available as part of a public dataset at
[telemetry.go.dev](https://telemetry.go.dev). Developers working on Go,
both inside and outside of Google, use this dataset to understand
how the Go toolchain is used and if it is performing as expected.

## Using the `gotelemetry` Command Line Tool {#command}

To manage Go Telemetry, you can use the `gotelemetry` command line tool.

	go install golang.org/x/telemetry/cmd/gotelemetry@latest

Here are some useful commands:

* `gotelemetry on`: Upload Go Telemetry data weekly.
* `gotelemetry off`: Do not upload Go Telemetry data. 
* `gotelemetry view`: View locally collected telemetry data.
* `gotelemetry clear`: Clear locally collected telemetry data at any time.

For the complete usage documentation of the gotelemetry command line tool, visit
[golang.org/x/telemetry/cmd/gotelemetry](https://golang.org/x/telemetry/cmd/gotelemetry).


## Approved Counters {#config}

Go Telemetry only uploads counters that have been approved through the [public proposal process](https://github.com/orgs/golang/projects/29).
You can find the set of approved counters as a Go module at
[golang.org/x/telemetry/config](https://go.googlesource.com/telemetry/+/refs/heads/master/config/config.json) and the [current config in use](https://telemetry.go.dev/config). 

## IDE Integration {#integration}

If you're using an integrated development environment (IDE) like Visual Studio
Code, versions
[`v0.14.0`](https://github.com/golang/tools/releases/tag/gopls%2Fv0.14.0) and
later of the Go language server [gopls](https://go.dev/s/gopls) collect
telemetry data. As described above, data is only uploaded after you have opted
in, either by using the command [`gotelemetry on`](#command) as described above
or by accepting a dialog in the IDE.

You can always opt out of uploading at any time by using the
[`gotelemetry local`](#command) or [`gotelemetry off`](#command) commands.

By sharing performance statistics, usage information, and crash reports with Go Telemetry,
you can help improve the Go programming language and its tools while also ensuring
your data privacy.
