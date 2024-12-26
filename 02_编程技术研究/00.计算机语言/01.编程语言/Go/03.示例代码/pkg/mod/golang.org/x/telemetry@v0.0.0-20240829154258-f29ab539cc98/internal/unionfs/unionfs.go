// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package unionfs allows multiple file systems to be read as a union.
package unionfs

import (
	"io/fs"
)

var _ fs.ReadDirFS = FS{}

// A FS is an FS presenting the union of the file systems in the slice. If
// multiple file systems provide a particular file, Open uses the FS listed
// earlier in the slice.
type FS []fs.FS

// Sub returns an FS corresponding to the merged subtree rooted at a set of
// fsys's dirs.
func Sub(fsys fs.FS, dirs ...string) (FS, error) {
	var subs FS
	for _, dir := range dirs {
		if _, err := fs.Stat(fsys, dir); err != nil {
			return nil, err
		}
		sub, err := fs.Sub(fsys, dir)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

func (fsys FS) Open(name string) (fs.File, error) {
	var errOut error
	for _, sub := range fsys {
		f, err := sub.Open(name)
		if err == nil {
			return f, nil
		}
		if errOut == nil {
			errOut = err
		}
	}
	return nil, errOut
}

func (fsys FS) ReadDir(name string) ([]fs.DirEntry, error) {
	var all []fs.DirEntry
	var seen map[string]bool // seen[name] is true if name is listed in all; lazily initialized
	var errOut error
	for _, sub := range fsys {
		list, err := fs.ReadDir(sub, name)
		if err != nil {
			errOut = err
		}
		if len(all) == 0 {
			all = append(all, list...)
		} else {
			if seen == nil {
				// Initialize seen only after we get two different directory listings.
				seen = make(map[string]bool)
				for _, d := range all {
					seen[d.Name()] = true
				}
			}
			for _, d := range list {
				name := d.Name()
				if !seen[name] {
					seen[name] = true
					all = append(all, d)
				}
			}
		}
	}
	if len(all) > 0 {
		return all, nil
	}
	return nil, errOut
}
