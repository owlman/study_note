// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unionfs

import (
	"io"
	"os"
	"reflect"
	"testing"
)

func TestFS_Open(t *testing.T) {
	fsys, err := Sub(os.DirFS("testdata"), "dir1", "dir2")
	if err != nil {
		t.Fatal(err)
	}

	type args struct {
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "file1 from dir1",
			args: args{
				name: "file1",
			},
			want:    "file 1 content from dir 1\n",
			wantErr: false,
		},
		{
			name: "file2 from dir2",
			args: args{
				name: "file2",
			},
			want:    "file 2 content\n",
			wantErr: false,
		},
		{
			name: "file not found",
			args: args{
				name: "file3",
			},
			want:    "file3",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := fsys.Open(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("FS.Open() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				bytes, err := io.ReadAll(file)
				if err != nil {
					t.Fatal(err)
				}
				got := string(bytes)
				if got != tt.want {
					t.Errorf("FS.Open() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestFS_ReadDir(t *testing.T) {
	var err error
	fsys, err := Sub(os.DirFS("testdata"), "dir1", "dir2")
	if err != nil {
		t.Fatal(err)
	}
	type args struct {
		name string
	}
	tests := []struct {
		name      string
		args      args
		fsys      FS
		wantFiles []string
	}{
		{
			name:      "",
			args:      args{"."},
			fsys:      fsys,
			wantFiles: []string{"file1", "file2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dirs, err := tt.fsys.ReadDir(tt.args.name)
			if err != nil {
				t.Errorf("FS.ReadDir() error = %v", err)
				return
			}
			var got []string
			for _, v := range dirs {
				got = append(got, v.Name())
			}
			if !reflect.DeepEqual(got, tt.wantFiles) {
				t.Errorf("FS.ReadDir() = %v, want %v", got, tt.wantFiles)
			}
		})
	}
}
