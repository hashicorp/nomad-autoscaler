//+build tools

// Package tools anonymously imports packages of tools used to build, test and
// lint Nomad Autoscaler. See the GNUMakefile for `go get` commands.
package tools

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "honnef.co/go/tools/cmd/staticcheck"
)
