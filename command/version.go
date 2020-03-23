package command

import (
	"fmt"
)

type VersionCommand struct {
	Version string
}

func (c *VersionCommand) Run(_ []string) int {
	fmt.Printf("Nomad Autoscaler %s", c.Version)
	return 0
}

func (c *VersionCommand) Synopsis() string {
	return "Prints the Nomad Autoscaler version"
}

func (c *VersionCommand) Help() string {
	return ""
}
