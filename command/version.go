// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import "fmt"

type VersionCommand struct {
	Version string
}

func (c *VersionCommand) Help() string {
	return ""
}

func (c *VersionCommand) Run(_ []string) int {
	fmt.Println(c.Version)
	return 0
}

func (c *VersionCommand) Synopsis() string {
	return "Prints the Nomad Autoscaler version"
}
