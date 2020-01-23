package main

import (
	"fmt"
	"os"

	"github.com/hashicorp/nomad-autoscaler/command"
	"github.com/mitchellh/cli"
)

func main() {
	c := cli.NewCLI("nomad-autoscaler", "0.1.0")
	c.Args = os.Args[1:]
	c.Commands = map[string]cli.CommandFactory{
		"run": func() (cli.Command, error) {
			return &command.RunCommand{}, nil
		},
	}

	exitCode, err := c.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing CLI: %v\n", err)
		os.Exit(1)
	}
	os.Exit(exitCode)
}
