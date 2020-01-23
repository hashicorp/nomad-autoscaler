package command

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/nomad-autoscaler/agent"
)

type RunCommand struct{}

type RunCommandArgs struct {
	ConfigPath string
}

// Help should return long-form help text that includes the command-line
// usage, a brief few sentences explaining the function of the command,
// and the complete list of flags the command accepts.
func (c *RunCommand) Help() string {
	helpText := `
Usage: nomad-autoscaler run [options] [args]

  This command starts a Nomad autoscaler instance.
`
	return strings.TrimSpace(helpText)
}

// Run should run the actual command with the given CLI instance and
// command-line arguments. It should return the exit status when it is
// finished.
//
// There are a handful of special exit codes this can return documented
// above that change behavior.
func (c *RunCommand) Run(args []string) int {
	// parse CLI args
	cArgs, err := c.parseFlags(args)
	if err != nil {
		log.Println(err)
		return 1
	}

	// load config file
	var config agent.Config
	if cArgs.ConfigPath != "" {
		err = hclsimple.DecodeFile(cArgs.ConfigPath, nil, &config)
		if err != nil {
			log.Printf("failed to read config file: %v", err)
			return 1
		}
	}

	// create and run agent
	a := agent.NewAgent(&config)
	err = a.Run()
	if err != nil {
		log.Printf("error: %v", err)
		return 1
	}
	return 0
}

// Synopsis should return a one-line, short synopsis of the command.
// This should be less than 50 characters ideally.
func (c *RunCommand) Synopsis() string {
	return "Run agent."
}

func (c *RunCommand) parseFlags(args []string) (*RunCommandArgs, error) {
	cArgs := &RunCommandArgs{}

	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.StringVar(&cArgs.ConfigPath, "config", "", "")

	err := flags.Parse(args)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CLI args: %v", err)
	}

	return cArgs, nil
}
