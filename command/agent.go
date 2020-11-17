package command

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-autoscaler/agent"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	agentHTTP "github.com/hashicorp/nomad-autoscaler/agent/http"
	flaghelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/flag"
)

type AgentCommand struct {
	args []string

	agent      *agent.Agent
	httpServer *agentHTTP.Server
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

  -nomad-ca-cert=<path>
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

Policy Options:

  -policy-dir=<path>
    The path to a directory used to load scaling policies.

  -policy-default-cooldown=<dur>
    The default cooldown that will be applied to all scaling policies which do
    not specify a cooldown period.

  -policy-default-evaluation-interval=<dur>
    The default evaluation interval that will be applied to all scaling policies
    which do not specify an evaluation interval.

Telemetry Options:

  -telemetry-disable-hostname
    Specifies whether gauge values should be prefixed with the local hostname.

  -telemetry-enable-hostname-label
    Enable adding hostname to metric labels.

  -telemetry-collection-interval=<dur>
    Specifies the time interval at which the agent collects telemetry data. The
    default is 1s.

  -telemetry-statsite-address=<addr>
    The address of the statsite aggregation server.

  -telemetry-statsd-address=<addr>
    The address of the statsd aggregation.

  -telemetry-dogstatsd-address=<addr>
    The address of the Datadog statsd server.

  -telemetry-dogstatsd-tag=<tag_list>
    A list of global tags that will be added to all telemetry packets sent to
    DogStatsD.

  -telemetry-prometheus-metrics
    Indicates whether the agent should make Prometheus formatted metrics available.
    Defaults to false.

  -telemetry-prometheus-retention-time=<dur>
    The time to retain Prometheus metrics before they are expired and untracked.

  -telemetry-circonus-api-token
    A valid API Token used to create/manage check. If provided, metric management
    is enabled.

  -telemetry-circonus-api-app
    The app name associated with API token. Defaults to nomad_autoscaler.

  -telemetry-circonus-api-url
    The base URL to use for contacting the Circonus API. Defaults to
    https://api.circonus.com/v2.

  -telemetry-circonus-submission-interval
    The interval at which metrics are submitted to Circonus. Defaults to 10s.

  -telemetry-circonus-submission-url
    The check.config.submission_url field from a previously created HTTPTRAP
    check.

  -telemetry-circonus-check-id
    The check id from a previously created HTTPTRAP check. The numeric portion
    of the check._cid field.

  -telemetry-circonus-check-force-metric-activation
    Force enabling metrics, as they are encountered, if the metric already exists
    and is NOT active. If check management is enabled, the default behavior is
    to add new metrics as they are encountered

  -telemetry-circonus-check-instance-id
    Uniquely identify the metrics coming from this agent. Defaults to hostname:app.

  -telemetry-circonus-check-search-tag
    A special tag that helps to narrow down the search results when neither a
    submission URL or check ID are provided. Defaults to service:app.

  -telemetry-circonus-check-tags
    A comma separated list of tags to apply to the check. The value of
    -telemetry-circonus-check-search-tag will always be added to the check.

  -telemetry-circonus-check-display-name
    The name used for the Circonus check that will be displayed in the UI. This
    defaults to the value of -telemetry-circonus-check-instance-id.

  -telemetry-circonus-broker-id
    The Circonus broker to use when creating a new check.

  -telemetry-circonus-broker-select-tag
    A tag which is used to select a broker ID when an explicit broker ID is not
    provided.
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

	parsedConfig := c.readConfig()
	if parsedConfig == nil {
		fmt.Println("Run 'nomad-autoscaler --help' for more information.")
		return 1
	}

	// Create the agent logger.
	logger := hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Name:       "agent",
		Level:      hclog.LevelFromString(parsedConfig.LogLevel),
		JSONFormat: parsedConfig.LogJson,
	})

	// create and run agent and HTTP server
	c.agent = agent.NewAgent(parsedConfig, logger)
	httpServer, err := agentHTTP.NewHTTPServer(parsedConfig.HTTP, logger, c.agent)
	if err != nil {
		logger.Error("failed to setup HTTP getHealth server", "error", err)
		return 1
	}

	c.httpServer = httpServer
	go c.httpServer.Start()
	defer c.httpServer.Stop()

	if err := c.agent.Run(); err != nil {
		logger.Error("failed to start agent", "error", err)
		return 1
	}
	return 0
}

func (c *AgentCommand) readConfig() *config.Agent {
	var configPath []string

	// cmdConfig is used to store any passed CLI flags.
	cmdConfig := &config.Agent{
		HTTP:      &config.HTTP{},
		Nomad:     &config.Nomad{},
		Policy:    &config.Policy{},
		Telemetry: &config.Telemetry{},
	}

	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	flags.Usage = func() { c.Help() }

	// Specify our top level CLI flags.
	flags.Var((*flaghelper.StringFlag)(&configPath), "config", "")
	flags.StringVar(&cmdConfig.LogLevel, "log-level", "", "")
	flags.BoolVar(&cmdConfig.LogJson, "log-json", false, "")
	flags.StringVar(&cmdConfig.PluginDir, "plugin-dir", "", "")

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

	// Specify our Policy CLI flags.
	flags.StringVar(&cmdConfig.Policy.Dir, "policy-dir", "", "")
	flags.Var((flaghelper.FuncDurationVar)(func(d time.Duration) error {
		cmdConfig.Policy.DefaultCooldown = d
		return nil
	}), "policy-default-cooldown", "")
	flags.Var((flaghelper.FuncDurationVar)(func(d time.Duration) error {
		cmdConfig.Policy.DefaultEvaluationInterval = d
		return nil
	}), "policy-default-evaluation-interval", "")

	// Specify our Telemetry CLI flags.
	flags.BoolVar(&cmdConfig.Telemetry.DisableHostname, "telemetry-disable-hostname", false, "")
	flags.BoolVar(&cmdConfig.Telemetry.EnableHostnameLabel, "telemetry-enable-hostname-label", false, "")
	flags.Var((flaghelper.FuncDurationVar)(func(d time.Duration) error {
		cmdConfig.Telemetry.CollectionInterval = d
		return nil
	}), "telemetry-collection-interval", "")
	flags.StringVar(&cmdConfig.Telemetry.StatsiteAddr, "telemetry-statsite-address", "", "")
	flags.StringVar(&cmdConfig.Telemetry.StatsdAddr, "telemetry-statsd-address", "", "")
	flags.StringVar(&cmdConfig.Telemetry.DogStatsDAddr, "telemetry-dogstatsd-address", "", "")
	flags.Var((*flaghelper.StringFlag)(&cmdConfig.Telemetry.DogStatsDTags), "telemetry-dogstatsd-tags", "")
	flags.BoolVar(&cmdConfig.Telemetry.PrometheusMetrics, "telemetry-prometheus-metrics", false, "")
	flags.Var((flaghelper.FuncDurationVar)(func(d time.Duration) error {
		cmdConfig.Telemetry.PrometheusRetentionTime = d
		return nil
	}), "telemetry-prometheus-retention-time", "")
	flags.StringVar(&cmdConfig.Telemetry.CirconusAPIToken, "telemetry-circonus-api-token", "", "")
	flags.StringVar(&cmdConfig.Telemetry.CirconusAPIApp, "telemetry-circonus-api-app", "", "")
	flags.StringVar(&cmdConfig.Telemetry.CirconusAPIURL, "telemetry-circonus-api-url", "", "")
	flags.StringVar(&cmdConfig.Telemetry.CirconusSubmissionInterval, "telemetry-circonus-submission-interval", "", "")
	flags.StringVar(&cmdConfig.Telemetry.CirconusCheckSubmissionURL, "telemetry-circonus-submission-url", "", "")
	flags.StringVar(&cmdConfig.Telemetry.CirconusCheckID, "telemetry-circonus-check-id", "", "")
	flags.StringVar(&cmdConfig.Telemetry.CirconusCheckForceMetricActivation, "telemetry-circonus-check-force-metric-activation", "", "")
	flags.StringVar(&cmdConfig.Telemetry.CirconusCheckInstanceID, "telemetry-circonus-check-instance-id", "", "")
	flags.StringVar(&cmdConfig.Telemetry.CirconusCheckSearchTag, "telemetry-circonus-check-search-tag", "", "")
	flags.StringVar(&cmdConfig.Telemetry.CirconusCheckTags, "telemetry-circonus-check-tags", "", "")
	flags.StringVar(&cmdConfig.Telemetry.CirconusCheckDisplayName, "telemetry-circonus-check-display-name", "", "")
	flags.StringVar(&cmdConfig.Telemetry.CirconusBrokerID, "telemetry-circonus-broker-id", "", "")
	flags.StringVar(&cmdConfig.Telemetry.CirconusBrokerSelectTag, "telemetry-circonus-broker-select-tag", "", "")

	if err := flags.Parse(c.args); err != nil {
		return nil
	}

	// Grab a default config as the base.
	cfg, err := config.Default()
	if err != nil {
		fmt.Printf("Error generating default agent config: %v\n", err)
		return nil
	}

	var validationErr *multierror.Error

	// Merge in the enterprise overlay.
	cfg = cfg.Merge(config.DefaultEntConfig())

	for _, path := range configPath {
		current, err := config.Load(path)
		if err != nil {
			fmt.Printf("Error loading configuration from %s: %s\n", path, err)
			return nil
		}

		if err := current.Validate(); err != nil {
			errPrefix := fmt.Sprintf("%s:", path)
			validationErr = multierror.Append(validationErr, multierror.Prefix(err, errPrefix))

			// Continue looping so we can validate other files.
			continue
		}

		if cfg == nil {
			cfg = current
		} else {
			cfg = cfg.Merge(current)
		}
	}

	if validationErr != nil {
		fmt.Printf("Invalid configuration. %v", validationErr)
		return nil
	}

	// Merge the read file based configuration with the passed CLI args.
	cfg = cfg.Merge(cmdConfig)

	return cfg
}
