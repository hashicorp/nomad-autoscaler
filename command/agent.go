package command

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad-autoscaler/agent/config"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent"
	flaghelper "github.com/hashicorp/nomad-autoscaler/helper/flag"
)

type AgentCommand struct {
	Ctx    context.Context
	Logger hclog.Logger

	args []string
}

// Help should return long-form help text that includes the command-line
// usage, a brief few sentences explaining the function of the command,
// and the complete list of flags the command accepts.
func (c *AgentCommand) Help() string {
	helpText := `
Usage: nomad-autoscaler agent [options] [args]

  Starts the Autoscaler agent and runs until an interrupt is received.

  The Nomad Autoscaler agent's configuration primarily comes from the config
  files used, but a subset of the options may also be passed directly as CLI
  arguments or environment variables, listed below.

Options:

  -config=<path>
    The path to either a single config file or a directory of config
    files to use for configuring the Nomad Autoscaler agent.

  -plugin-dir=<path>
    The plugin directory is used to discover Nomad Autoscaler plugins.

  -scan-interval=<dur>
    The time to wait between Nomad Autoscaler evaluations.

Nomad Options:

  -nomad-address=<addr>
    The address of the Nomad server.

  nomad-region=<identifier>
    The region of the Nomad servers to connect with.
`
	return strings.TrimSpace(helpText)
}

// Synopsis should return a one-line, short synopsis of the command.
// This should be less than 50 characters ideally.
func (c *AgentCommand) Synopsis() string {
	return "Runs a Nomad autoscaler agent"
}

// Run should run the actual command with the given CLI instance and
// command-line arguments. It should return the exit status when it is
// finished.
//
// There are a handful of special exit codes this can return documented
// above that change behavior.
func (c *AgentCommand) Run(args []string) int {

	c.args = args

	parsedConfig, err := c.readConfig()
	if err != nil {
		c.Logger.Error("failed to parse command arguments", "error", err)
		fmt.Print(c.Help())
		return 1
	}

	// create and run agent
	a := agent.NewAgent(parsedConfig, c.Logger.Named("agent"))
	if err = a.Run(c.Ctx); err != nil {
		c.Logger.Error("failed to start agent", "error", err)
		return 1
	}
	return 0
}

func (c *AgentCommand) readConfig() (*config.Agent, error) {
	var configPath []string

	// cmdConfig is used to store any passed CLI flags.
	cmdConfig := &config.Agent{
		Nomad: &config.Nomad{},
	}

	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	flags.Usage = func() { c.Help() }

	// Specify our top level CLI flags.
	flags.Var((*flaghelper.StringFlag)(&configPath), "config", "")
	flags.StringVar(&cmdConfig.PluginDir, "plugin-dir", "", "")
	flags.Var((flaghelper.FuncDurationVar)(func(d time.Duration) error {
		cmdConfig.ScanInterval = d
		return nil
	}), "scan-interval", "")

	// Specify or Nomad client CLI flags.
	flags.StringVar(&cmdConfig.Nomad.Address, "nomad-address", "", "")
	flags.StringVar(&cmdConfig.Nomad.Region, "nomad-region", "", "")

	if err := flags.Parse(c.args); err != nil {
		return nil, err
	}

	// Grab a default config as the base.
	cfg, err := config.Default()
	if err != nil {
		return nil, fmt.Errorf("failed to generate default agent config: %v", err)
	}

	for _, path := range configPath {
		current, err := config.Load(path)
		if err != nil {
			return nil, fmt.Errorf("error loading configuration from %s: %s", path, err)
		}

		if cfg == nil {
			cfg = current
		} else {
			cfg = cfg.Merge(current)
		}
	}

	// Merge the read file based configuration with the passed CLI args.
	cfg = cfg.Merge(cmdConfig)

	return cfg, nil
}
