// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package file

import (
	"fmt"
	"io"
	"os"
	"strings"

	"path/filepath"
)

func GetFileListFromDir(dir string, suffixes ...string) ([]string, error) {

	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("configuration path must be a directory: %s", dir)
	}

	var files []string
	err = nil
	for err != io.EOF {
		var fis []os.FileInfo
		fis, err = f.Readdir(128)
		if err != nil && err != io.EOF {
			return nil, err
		}

		for _, fi := range fis {
			// Ignore directories
			if fi.IsDir() {
				continue
			}

			// Only care about files that are valid to load.
			name := fi.Name()

			if suffixes != nil {
				if !fileHasSuffix(name, suffixes) || IsTemporaryFile(name) {
					continue
				}
				path := filepath.Join(dir, name)
				files = append(files, path)
			}
		}
	}
	return files, nil
}

func fileHasSuffix(file string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(file, suffix) {
			return true
		}
	}
	return false
}
