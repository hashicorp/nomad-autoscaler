//go:build !windows
// +build !windows

package manager

import "os"

// executable checks to see if the file is executable by anyone.
func executable(_ string, f os.FileInfo) bool {
	return f.Mode().Perm()&0111 != 0
}
