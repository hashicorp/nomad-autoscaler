// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows
// +build windows

package manager

import (
	"os"
	"path/filepath"
)

// On windows, an executable can be any file with any extension. To avoid
// introspecting the file, here we skip executability checks on windows systems
// and simply check for the convention of an `.exe` extension.
func executable(path string, _ os.FileInfo) bool {
	return filepath.Ext(path) == ".exe"
}
