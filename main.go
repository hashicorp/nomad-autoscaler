// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"os"

	"github.com/hashicorp/nomad-autoscaler/command"
	"github.com/hashicorp/nomad-autoscaler/version"
	"github.com/mitchellh/cli"
)

func main() {

	versionString := fmt.Sprintf("Nomad Autoscaler %s", version.GetHumanVersion())
	c := cli.NewCLI("nomad-autoscaler", versionString)
	c.Args = os.Args[1:]
	c.Commands = map[string]cli.CommandFactory{
		"agent": func() (cli.Command, error) {
			return &command.AgentCommand{}, nil
		},
		"version": func() (cli.Command, error) {
			return &command.VersionCommand{Version: versionString}, nil
		},
	}

	exitCode, err := c.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing CLI: %v\n", err)
	}
	os.Exit(exitCode)
}
