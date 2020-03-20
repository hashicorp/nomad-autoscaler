package command

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	flaghelper "github.com/hashicorp/nomad-autoscaler/helper/flag"
)

type AgentCommand struct {
	Ctx context.Context

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

  -log-level=<level>
    Specify the verbosity level of Nomad Autoscaler's logs. Valid values
    include DEBUG, INFO, and WARN, in decreasing order of verbosity. The
    default is INFO.

  -log-json
    Output logs in a JSON format. The default is false.

  -plugin-dir=<path>
    The plugin directory is used to discover Nomad Autoscaler plugins. If not
    specified, the plugin directory defaults to be that of
    <current-dir>/plugins/.

  -scan-interval=<dur>
    The time to wait between Nomad Autoscaler evaluations.

HTTP Options:

  -http-bind-address=<addr>
    The HTTP address that the health server will bind to. The default is
    127.0.0.1.

  -http-bind-port=<port>
    The port that the health server will bind to. The default is 8080.

Nomad Options:

  -nomad-address=<addr>
    The address of the Nomad server in the form of protocol://addr:port. The
    default is http://127.0.0.1:4646.

  -nomad-region=<region>
    The region of the Nomad servers to connect with.

  -nomad-namespace=<namespace>
    The target namespace for queries and actions bound to a namespace.

  -nomad-token=<token>
    The SecretID of an ACL token to use to authenticate API requests with.

  -nomad-http-auth=<username:password>
    The authentication information to use when connecting to a Nomad API which
    is using HTTP authentication.

  -ca-cert=<path>
    Path to a PEM encoded CA cert file to use to verify the Nomad server SSL
    certificate.

  -nomad-ca-path=<path>
    Path to a directory of PEM encoded CA cert files to verify the Nomad server
    SSL certificate. If both -nomad-ca-cert and -nomad-ca-path are specified,
    -nomad-ca-cert is used.

  -nomad-client-cert=<path>
    Path to a PEM encoded client certificate for TLS authentication to the
    Nomad server. Must also specify -nomad-client-key.

  -nomad-client-key=<path>
    Path to an unencrypted PEM encoded private key matching the client
    certificate from -nomad-client-cert.

  -nomad-tls-server-name=<name>
    The server name to use as the SNI host when connecting via TLS.

  -nomad-skip-verify
    Do not verify TLS certificates. This is strongly discouraged.
  
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
		fmt.Printf("Error parsing command arguments: %v", err)
		fmt.Print(c.Help())
		return 1
	}

	// Create the agent logger.
	logger := hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Name:       "agent",
		Level:      hclog.LevelFromString(parsedConfig.LogLevel),
		JSONFormat: parsedConfig.LogJson,
	})

	// create and run agent
	a := agent.NewAgent(parsedConfig, logger)
	if err = a.Run(c.Ctx); err != nil {
		logger.Error("failed to start agent", "error", err)
		return 1
	}
	return 0
}

func (c *AgentCommand) readConfig() (*config.Agent, error) {
	var configPath []string

	// cmdConfig is used to store any passed CLI flags.
	cmdConfig := &config.Agent{
		HTTP:  &config.HTTP{},
		Nomad: &config.Nomad{},
	}

	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	flags.Usage = func() { c.Help() }

	// Specify our top level CLI flags.
	flags.Var((*flaghelper.StringFlag)(&configPath), "config", "")
	flags.StringVar(&cmdConfig.LogLevel, "log-level", "", "")
	flags.BoolVar(&cmdConfig.LogJson, "log-json", false, "")
	flags.StringVar(&cmdConfig.PluginDir, "plugin-dir", "", "")
	flags.Var((flaghelper.FuncDurationVar)(func(d time.Duration) error {
		cmdConfig.ScanInterval = d
		return nil
	}), "scan-interval", "")

	// Specify our HTTP bind flags.
	flags.StringVar(&cmdConfig.HTTP.BindAddress, "http-bind-address", "", "")
	flags.IntVar(&cmdConfig.HTTP.BindPort, "http-bind-port", 0, "")

	// Specify our Nomad client CLI flags.
	flags.StringVar(&cmdConfig.Nomad.Address, "nomad-address", "", "")
	flags.StringVar(&cmdConfig.Nomad.Region, "nomad-region", "", "")
	flags.StringVar(&cmdConfig.Nomad.Namespace, "nomad-namespace", "", "")
	flags.StringVar(&cmdConfig.Nomad.Token, "nomad-token", "", "")
	flags.StringVar(&cmdConfig.Nomad.HTTPAuth, "nomad-http-auth", "", "")
	flags.StringVar(&cmdConfig.Nomad.CACert, "nomad-ca-cert", "", "")
	flags.StringVar(&cmdConfig.Nomad.CAPath, "nomad-ca-path", "", "")
	flags.StringVar(&cmdConfig.Nomad.ClientCert, "nomad-client-cert", "", "")
	flags.StringVar(&cmdConfig.Nomad.ClientKey, "nomad-client-key", "", "")
	flags.StringVar(&cmdConfig.Nomad.TLSServerName, "nomad-tls-server-name", "", "")
	flags.BoolVar(&cmdConfig.Nomad.SkipVerify, "nomad-skip-verify", false, "")

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
