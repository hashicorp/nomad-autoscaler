// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package file

import "strings"

// IsTemporaryFile returns true or false depending on whether the provided file
// name is a temporary file for the following editors: emacs or vim.
func IsTemporaryFile(name string) bool {
	return strings.HasSuffix(name, "~") || // vim
		strings.HasPrefix(name, ".#") || // emacs
		(strings.HasPrefix(name, "#") && strings.HasSuffix(name, "#")) // emacs
}
