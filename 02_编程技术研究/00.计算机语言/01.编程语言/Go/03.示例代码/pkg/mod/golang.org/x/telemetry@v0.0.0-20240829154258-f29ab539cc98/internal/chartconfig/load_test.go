// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package chartconfig_test

import (
	"reflect"
	"testing"

	"golang.org/x/telemetry/internal/chartconfig"
)

func TestLoad(t *testing.T) {
	// Test that we can actually load the chart config.
	if _, err := chartconfig.Load(); err != nil {
		t.Errorf("Load() failed: %v", err)
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []chartconfig.ChartConfig
	}{
		{"empty", "", nil},
		{"single field", "title: A", []chartconfig.ChartConfig{{Title: "A"}}},
		{
			"basic", `
title: A
description: B
type: C
program: D
counter: E
issue: F1
issue: F2
depth: 2
error: 0.1
version: v2.0.0
`,
			[]chartconfig.ChartConfig{{
				Title:       "A",
				Description: "B",
				Type:        "C",
				Program:     "D",
				Counter:     "E",
				Issue:       []string{"F1", "F2"},
				Depth:       2,
				Error:       0.1,
				Version:     "v2.0.0",
			}},
		},
		{
			"partial", `
title: A
description: B
`,
			[]chartconfig.ChartConfig{
				{Title: "A", Description: "B"},
			},
		},
		{
			"comments and whitespace", `
# A comment
title: A # a line comment

# Another comment

description: B


`,
			[]chartconfig.ChartConfig{
				{Title: "A", Description: "B"},
			},
		},
		{
			"multi-record", `
# Empty records are skipped
---
title: A
description: B

---

title: C
description: D
`,
			[]chartconfig.ChartConfig{
				{Title: "A", Description: "B"},
				{Title: "C", Description: "D"},
			},
		},
		{
			"example", `
title: Editor Distribution
counter: gopls/editor:{emacs,vim,vscode,other}
description: measure editor distribution for gopls users.
type: partition
issue: TBD
program: golang.org/x/tools/gopls
`,
			[]chartconfig.ChartConfig{
				{
					Title:       "Editor Distribution",
					Description: "measure editor distribution for gopls users.",
					Counter:     "gopls/editor:{emacs,vim,vscode,other}",
					Type:        "partition",
					Issue:       []string{"TBD"},
					Program:     "golang.org/x/tools/gopls",
				},
			},
		},
		{
			"multiline counter field", `
counter: foo:{
	bar,
    baz
}
`,
			[]chartconfig.ChartConfig{
				{Counter: "foo:{bar,baz}"},
			},
		},
		{
			"multiline counter field with braces immediately next to text", `
counter: foo:{bar,
 baz}
`,
			[]chartconfig.ChartConfig{
				{Counter: "foo:{bar,baz}"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := chartconfig.Parse([]byte(test.input))
			if err != nil {
				t.Fatalf("Parse(...) failed: %v", err)
			}
			if len(got) != len(test.want) {
				t.Fatalf("Parse(...) returned %d records, want %d", len(got), len(test.want))
			}
			for i, got := range got {
				want := test.want[i]
				if !reflect.DeepEqual(got, want) {
					t.Errorf("Parse(...): record %d = %#v, want %#v", i, got, want)
				}
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			"leading space",
			`
 title: foo
`,
		},
		{
			"unknown key",
			`
foo: bar
`,
		},
		{
			"bad separator",
			`
title: foo
--- # comments aren't allowed after separators
title: bar
`,
		},
		{
			"invalid depth",
			`
depth: notanint
`,
		},
		{
			"open curly brace not in counter field",
			`
title: {
`,
		},
		{
			"close curly brace not in counter field",
			`
title: }
`,
		},
		{
			"end of record within multiline counter field",
			`
counter: foo{
  bar
---
title: baz
`,
		},
		{
			"end of file within multiline counter field",
			`
counter: foo{
  bar
`,
		},
		{
			"close curly before open curly",
			`
counter: }foo{
  bar
}`,
		},
		{
			"open curly after close curly",
			`
counter: foo{
  bar
} {`,
		},
		{
			"open curly after open curly same line",
			`
counter: foo{{
  bar
}`,
		},
		{
			"open curly after open curly different line",
			`
counter: foo{
  {bar
}`,
		},
		{
			"close curly after close curly",
			`
counter: foo{
  bar
} }`,
		},
		{
			"comma right before close curly",
			`
counter: foo{
  bar,
  baz,
} }`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := chartconfig.Parse([]byte(test.input))
			if err == nil {
				t.Fatalf("Parse(...) succeeded unexpectedly")
			}
		})
	}
}
