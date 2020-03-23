package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hashicorp/nomad-autoscaler/command"
	"github.com/hashicorp/nomad-autoscaler/version"
	"github.com/mitchellh/cli"
)

func main() {
	// create context to handle signals
	ctx, cancel := context.WithCancel(context.Background())

	signalCn := make(chan os.Signal, 1)
	signal.Notify(signalCn, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalCn
		cancel()
	}()

	c := cli.NewCLI("nomad-autoscaler", "0.1.0")
	c.Args = os.Args[1:]
	c.Commands = map[string]cli.CommandFactory{
		"agent": func() (cli.Command, error) {
			return &command.AgentCommand{Ctx: ctx}, nil
		},
		"version": func() (cli.Command, error) {
			return &command.VersionCommand{Version: version.GetHumanVersion()}, nil
		},
	}

	exitCode, err := c.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing CLI: %v\n", err)
		os.Exit(1)
	}
	os.Exit(exitCode)
}
