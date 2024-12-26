// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The view command is a server intended to be run on a users machine to
// display the local counters and time series charts of counters.
package view

import (
	"fmt"
	"html/template"
	"reflect"
	"testing"
	"time"

	"golang.org/x/telemetry/internal/config"
	"golang.org/x/telemetry/internal/telemetry"
)

func Test_summary(t *testing.T) {
	type args struct {
		cfg    *config.Config
		meta   map[string]string
		counts map[string]uint64
	}
	cfg := config.NewConfig(&telemetry.UploadConfig{
		GOOS:      []string{"linux"},
		GOARCH:    []string{"amd64"},
		GoVersion: []string{"go1.20.1"},
		Programs: []*telemetry.ProgramConfig{
			{
				Name:     "gopls",
				Versions: []string{"v1.2.3"},
				Counters: []telemetry.CounterConfig{
					{Name: "editor"},
				},
			},
		},
	})
	tests := []struct {
		name string
		args args
		want template.HTML
	}{
		{
			"empty summary",
			args{
				cfg:    cfg,
				meta:   map[string]string{"Program": "gopls", "Version": "v1.2.3", "GOOS": "linux", "GOARCH": "amd64", "GoVersion": "go1.20.1"},
				counts: map[string]uint64{"editor": 10},
			},
			template.HTML(""),
		},
		{
			"empty config/unknown program",
			args{
				cfg:    config.NewConfig(&telemetry.UploadConfig{}),
				meta:   map[string]string{"Program": "gopls", "Version": "v1.2.3", "GOOS": "linux", "GOARCH": "amd64", "GoVersion": "go1.20.1"},
				counts: map[string]uint64{"editor": 10},
			},
			template.HTML("The program <code>gopls</code> is unregistered. No data from this set would be uploaded to the Go team."),
		},
		{
			"unknown counter",
			args{
				cfg:    cfg,
				meta:   map[string]string{"Program": "gopls", "Version": "v1.2.3", "GOOS": "linux", "GOARCH": "amd64", "GoVersion": "go1.20.1"},
				counts: map[string]uint64{"editor": 10, "foobar": 10},
			},
			template.HTML("Unregistered counter(s) <code>foobar</code> would be excluded from a report. "),
		},
		{
			"unknown goos",
			args{
				cfg:    cfg,
				meta:   map[string]string{"Program": "gopls", "Version": "v1.2.3", "GOOS": "windows", "GOARCH": "arm64", "GoVersion": "go1.20.1"},
				counts: map[string]uint64{"editor": 10, "foobar": 10},
			},
			template.HTML("The GOOS/GOARCH combination <code>windows/arm64</code>  is unregistered. No data from this set would be uploaded to the Go team."),
		},
		{
			"multiple unknown fields",
			args{
				cfg:    cfg,
				meta:   map[string]string{"Program": "gopls", "Version": "v1.2.5", "GOOS": "linux", "GOARCH": "amd64", "GoVersion": "go1.25.1"},
				counts: map[string]uint64{"editor": 10, "foobar": 10},
			},
			template.HTML("The go version <code>go1.25.1</code>  is unregistered. No data from this set would be uploaded to the Go team."),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summary(tt.args.cfg, tt.args.meta, tt.args.counts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("summary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_reportsDomain(t *testing.T) {
	mustParseDate := func(date string) time.Time {
		ts, err := time.Parse(telemetry.DateOnly, date)
		if err != nil {
			t.Fatalf("failed to parse date %q: %v", date, err)
		}
		return ts
	}

	tests := []struct {
		name        string
		reportDates []string
		want        [2]time.Time
		wantErr     bool
	}{
		{
			name:    "empty",
			wantErr: true,
		},
		{
			name:        "one",
			reportDates: []string{"2024-01-08"},
			want:        [2]time.Time{mustParseDate("2024-01-01"), mustParseDate("2024-01-08")},
		},
		{
			name:        "two",
			reportDates: []string{"2024-04-08", "2024-06-01"},
			want:        [2]time.Time{mustParseDate("2024-04-01"), mustParseDate("2024-06-01")},
		},
		{
			name:        "three",
			reportDates: []string{"2024-04-08", "2024-01-08", "2024-06-01"},
			want:        [2]time.Time{mustParseDate("2024-01-01"), mustParseDate("2024-06-01")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reports := make([]*telemetryReport, len(tt.reportDates))
			for i, date := range tt.reportDates {
				weekEnd, err := parseReportDate(date)
				if err != nil {
					t.Fatalf("parseReport(%v) failed: %v", date, err)
				}
				reports[i] = &telemetryReport{
					WeekEnd: weekEnd,
					ID:      fmt.Sprintf("report-%d", i),
				}
			}
			got, err := reportsDomain(reports)
			if tt.wantErr && err == nil ||
				err == nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("reportsDomain() = (%v, %v), want (%v, err=%v)", got, err, tt.want, tt.wantErr)
			}
		})
	}
}
