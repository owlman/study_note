// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package chartconfig

import (
	_ "embed"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

//go:embed config.txt
var chartConfig []byte

func Raw() []byte {
	return chartConfig
}

// Load loads and parses the current chart config.
func Load() ([]ChartConfig, error) {
	return Parse(chartConfig)
}

// Parse parses ChartConfig records from the provided raw data, returning an
// error if the config has invalid syntax. See the package documentation for a
// description of the record syntax.
//
// Even with correct syntax, the resulting chart config may not meet all the
// requirements described in the package doc. Call [Validate] to check whether
// the config data is coherent.
func Parse(data []byte) ([]ChartConfig, error) {
	// Collect field information for the record type.
	var (
		prefixes []string                               // for parse errors
		fields   = make(map[string]reflect.StructField) // key -> struct field
	)
	{
		typ := reflect.TypeOf(ChartConfig{})
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			key := strings.ToLower(f.Name)
			if _, ok := fieldParsers[key]; !ok {
				panic(fmt.Sprintf("no parser for field %q", f.Name))
			}
			prefixes = append(prefixes, "'"+key+":'")
			fields[key] = f
		}
		sort.Strings(prefixes)
	}

	// Read records, separated by '---'
	var (
		records    []ChartConfig
		inProgress = new(ChartConfig)      // record value currently being parsed
		set        = make(map[string]bool) // fields that are set so far; empty records are skipped
	)
	flushRecord := func() {
		if len(set) > 0 { // only flush non-empty records
			records = append(records, *inProgress)
		}
		inProgress = new(ChartConfig)
		set = make(map[string]bool)
	}

	// Within bucket braces in counter fields, newlines are ignored.
	// if we're in the middle of a multiline counter field, accumulatedCounterText
	// contains the joined lines of the field up to the current line. Once
	// a line containing an end brace is reached, line will be set to the
	// joined lines of accumulatedCounterText and processed as a single line.
	var accumulatedCounterText string

	for lineNum, line := range strings.Split(string(data), "\n") {
		if line == "---" {
			if accumulatedCounterText != "" {
				return nil, fmt.Errorf("line %d: reached end of record while processing multiline counter field", lineNum)
			}
			flushRecord()
			continue
		}
		text, _, _ := strings.Cut(line, "#") // trim comments

		// Processing of counter fields which can appear across multiple lines.
		// See comment on accumulatedCounterText.
		if accumulatedCounterText == "" {
			if oi := strings.Index(text, "{"); oi >= 0 {
				if strings.Contains(text[:oi], "}") {
					return nil, fmt.Errorf("line %d: invalid line %q: unexpected '}'", lineNum, line)
				}
				if strings.Contains(text[oi+len("{"):], "{") {
					return nil, fmt.Errorf("line %d: invalid line %q: unexpected '{'", lineNum, line)
				}
				if !strings.HasPrefix(text, "counter:") {
					return nil, fmt.Errorf("line %d: invalid line %q: '{' is only allowed to appear within a counter field", lineNum, line)
				}
				accumulatedCounterText = strings.TrimRightFunc(text, unicode.IsSpace)
				// Don't continue here. If the counter field is a single line
				// the check for the close brace below will close the line
				// and process it as text. Set text to "" so when it's appended to
				// accumulatedCounterText we don't add the line twice.
				text = ""
			} else if strings.Contains(text, "}") {
				return nil, fmt.Errorf("line %d: invalid line %q: unexpected '}'", lineNum, line)
			}
		}
		if accumulatedCounterText != "" {
			if strings.Contains(text, "{") {
				return nil, fmt.Errorf("line %d: invalid line %q: '{' is only allowed to appear once within a counter field", lineNum, line)
			}
			accumulatedCounterText += strings.TrimSpace(text)
			if ci := strings.Index(accumulatedCounterText, "}"); ci >= 0 {
				if strings.Contains(accumulatedCounterText[ci+len("}"):], "}") {
					return nil, fmt.Errorf("line %d: invalid line %q: unexpected '}'", lineNum, line)
				}
				if ci > 0 && strings.HasSuffix(accumulatedCounterText[:ci], ",") {
					return nil, fmt.Errorf("line %d: invalid line %q: unexpected '}' after ','", lineNum, line)
				}
				text = accumulatedCounterText
				accumulatedCounterText = ""
			} else {
				// We're in the middle of a multiline counter field. Continue
				// processing.
				continue
			}
		}

		var key string
		for k := range fields {
			prefix := k + ":"
			if strings.HasPrefix(text, prefix) {
				key = k
				text = text[len(prefix):]
				break
			}
		}

		text = strings.TrimSpace(text)
		if text == "" {
			// Check for empty lines before the field == nil check below.
			// Lines consisting only of whitespace and comments are OK.
			continue
		}
		if key == "" {
			return nil, fmt.Errorf("line %d: invalid line %q: lines must be '---', consist only of whitespace/comments, or start with %s", lineNum, line, strings.Join(prefixes, ", "))
		}
		field := fields[key]
		v := reflect.ValueOf(inProgress).Elem().FieldByName(field.Name)
		if set[key] && field.Type.Kind() != reflect.Slice {
			return nil, fmt.Errorf("line %d: field %s may not be repeated", lineNum, strings.ToLower(field.Name))
		}
		parser := fieldParsers[key]
		if err := parser(v, text); err != nil {
			return nil, fmt.Errorf("line %d: field %q: %v", lineNum, field.Name, err)
		}
		set[key] = true
	}

	if accumulatedCounterText != "" {
		return nil, fmt.Errorf("reached end of file while processing multiline counter field")
	}

	flushRecord()
	return records, nil
}

// A fieldParser parses the provided input and writes to v, which must be
// addressable.
type fieldParser func(v reflect.Value, input string) error

var fieldParsers = map[string]fieldParser{
	"title":       parseString,
	"description": parseString,
	"issue":       parseSlice(parseString),
	"type":        parseString,
	"program":     parseString,
	"counter":     parseString,
	"depth":       parseInt,
	"error":       parseFloat,
	"version":     parseString,
}

func parseString(v reflect.Value, input string) error {
	v.SetString(input)
	return nil
}

func parseInt(v reflect.Value, input string) error {
	i, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid int value %q", input)
	}
	v.SetInt(i)
	return nil
}

func parseFloat(v reflect.Value, input string) error {
	f, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return fmt.Errorf("invalid float value %q", input)
	}
	v.SetFloat(f)
	return nil
}

func parseSlice(elemParser fieldParser) fieldParser {
	return func(v reflect.Value, input string) error {
		elem := reflect.New(v.Type().Elem()).Elem()
		v.Set(reflect.Append(v, elem))
		elem = v.Index(v.Len() - 1)
		if err := elemParser(elem, input); err != nil {
			return err
		}
		return nil
	}
}
