// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The chartconfig package defines the ChartConfig type, representing telemetry
// chart configuration, as well as utilities for parsing and validating this
// configuration.
//
// Chart configuration defines the set of aggregations active on the telemetry
// server, and are used to derive which data needs to be uploaded by users.
// See the original blog post for more details:
//
//	https://research.swtch.com/telemetry-design#configuration
//
// The record format defined in this package differs slightly from that of the
// blog post. This format is still experimental, and subject to change.
//
// Configuration records consist of fields, comments, and whitespace. A field
// is defined by a line starting with a valid key, followed immediately by ":",
// and then a textual value, which cannot include the comment separator '#'.
//
// Comments start with '#', and extend to the end of the line.
//
// The following keys are supported. Any entry not marked as (optional) must be
// provided.
//
//   - title: the chart title.
//   - description: (optional) a longer description of the chart.
//   - issue: a go issue tracker URL proposing the chart configuration.
//     Multiple issues may be provided by including additional 'issue:' lines.
//     All proposals must be in the 'accepted' state.
//   - type: the chart type: currently only partition, histogram, and stack are
//     supported.
//   - program: the package path of the program for which this chart applies.
//   - version: (optional) the first version for which this chart applies. Must
//     be a valid semver value.
//   - counter: the primary counter this chart illustrates, including buckets
//     for histogram and partition charts.
//   - depth: (optional) stack counters only; the maximum stack depth to collect
//   - error: (optional) the desired error rate for this chart, which
//     determines collection rate
//
// Multiple records are separated by "---" lines.
//
// For example:
//
//	# This config defines an ordinary counter.
//	counter: gopls/editor:{emacs,vim,vscode,other} # TODO(golang/go#34567): add more editors
//	title: Editor Distribution
//	description: measure editor distribution for gopls users.
//	type: partition
//	issue: https://go.dev/issue/12345
//	program: golang.org/x/tools/gopls
//	version: v1.0.0
//	version: [v2.0.0, v2.3.4]
//	version: [v3.0.0, ]
//
//	---
//
//	# This config defines a stack counter.
//	counter: gopls/bug
//	title: Gopls bug reports.
//	description: Stacks of bugs encountered on the gopls server.
//	issue: https://go.dev/12345
//	issue: https://go.dev/23456 # increase stack depth
//	type: stack
//	program: golang.org/x/tools/gopls
//	depth: 10
package chartconfig

// A ChartConfig defines the configuration for a single chart/collection on the
// telemetry server.
//
// See the package documentation for field definitions.
type ChartConfig struct {
	Title       string
	Description string
	Issue       []string
	Type        string
	Program     string
	Counter     string
	Depth       int
	Error       float64 // TODO(rfindley) is Error still useful?
	Version     string
}
