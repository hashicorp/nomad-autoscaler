package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	agentServer "github.com/hashicorp/nomad-autoscaler/agent/http"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	"github.com/hashicorp/nomad-autoscaler/policy"
	filePolicy "github.com/hashicorp/nomad-autoscaler/policy/file"
	nomadPolicy "github.com/hashicorp/nomad-autoscaler/policy/nomad"
	"github.com/hashicorp/nomad/api"
)

type Agent struct {
	logger        hclog.Logger
	config        *config.Agent
	nomadClient   *api.Client
	pluginManager *manager.PluginManager
	policyManager *policy.Manager
	httpServer    *agentServer.Server
}

func NewAgent(c *config.Agent, logger hclog.Logger) *Agent {
	return &Agent{
		logger: logger,
		config: c,
	}
}

func (a *Agent) Run() error {
	defer a.stop()

	// Create context to handle propagation to downstream routines.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Generate the Nomad client.
	if err := a.generateNomadClient(); err != nil {
		return err
	}

	// launch plugins
	if err := a.setupPlugins(); err != nil {
		return fmt.Errorf("failed to setup plugins: %v", err)
	}

	// Setup the telemetry sinks.
	inMem, err := a.setupTelemetry(a.config.Telemetry)
	if err != nil {
		return fmt.Errorf("failed to setup telemetry: %v", err)
	}

	// Setup and start the HTTP server.
	httpServer, err := agentServer.NewHTTPServer(a.config.HTTP, a.logger, inMem)
	if err != nil {
		return fmt.Errorf("failed to setup HTTP getHealth server: %v", err)
	}

	a.httpServer = httpServer
	go a.httpServer.Start()

	policyEvalCh := a.setupPolicyManager()
	go a.policyManager.Run(ctx, policyEvalCh, a.config.Telemetry.CollectionInterval)

	// Launch the eval handler.
	go a.runEvalHandler(ctx, policyEvalCh)

	// Wait for our exit.
	a.handleSignals()
	return nil
}

func (a *Agent) runEvalHandler(ctx context.Context, evalCh chan *policy.Evaluation) {
	for {
		select {
		case <-ctx.Done():
			a.logger.Info("context closed, shutting down eval handler")
			return
		case policyEval := <-evalCh:
			w := policy.NewWorker(a.logger, a.pluginManager, a.policyManager)
			go w.HandlePolicy(ctx, policyEval.Policy)
		}
	}
}

func (a *Agent) setupPolicyManager() chan *policy.Evaluation {

	// Create our processor, a shared method for performing basic policy
	// actions.
	cfgDefaults := policy.ConfigDefaults{
		DefaultEvaluationInterval: a.config.Policy.DefaultEvaluationInterval,
		DefaultCooldown:           a.config.Policy.DefaultCooldown,
	}
	policyProcessor := policy.NewProcessor(&cfgDefaults, a.getNomadAPMNames())

	// Setup our initial default policy source which is Nomad.
	sources := map[policy.SourceName]policy.Source{
		policy.SourceNameNomad: nomadPolicy.NewNomadSource(a.logger, a.nomadClient, policyProcessor),
	}

	// If the operators has configured a scaling policy directory to read from
	// then setup the file source.
	if a.config.Policy.Dir != "" {
		sources[policy.SourceNameFile] = filePolicy.NewFileSource(a.logger, a.config.Policy.Dir, policyProcessor)
	}

	a.policyManager = policy.NewManager(a.logger, sources, a.pluginManager)

	return make(chan *policy.Evaluation, 10)
}

func (a *Agent) stop() {
	// Stop the health server.
	if a.httpServer != nil {
		a.httpServer.Stop()
	}

	// Kill all the plugins.
	if a.pluginManager != nil {
		a.pluginManager.KillPlugins()
	}
}

// generateNomadClient takes the internal Nomad configuration, translates and
// merges it into a Nomad API config object and creates a client.
func (a *Agent) generateNomadClient() error {

	// Use the Nomad API default config which gets populated by defaults and
	// also checks for environment variables.
	cfg := api.DefaultConfig()

	// Merge our top level configuration options in.
	if a.config.Nomad.Address != "" {
		cfg.Address = a.config.Nomad.Address
	}
	if a.config.Nomad.Region != "" {
		cfg.Region = a.config.Nomad.Region
	}
	if a.config.Nomad.Namespace != "" {
		cfg.Namespace = a.config.Nomad.Namespace
	}
	if a.config.Nomad.Token != "" {
		cfg.SecretID = a.config.Nomad.Token
	}

	// Merge HTTP auth.
	if a.config.Nomad.HTTPAuth != "" {
		cfg.HttpAuth = nomadHelper.HTTPAuthFromString(a.config.Nomad.HTTPAuth)
	}

	// Merge TLS.
	if a.config.Nomad.CACert != "" {
		cfg.TLSConfig.CACert = a.config.Nomad.CACert
	}
	if a.config.Nomad.CAPath != "" {
		cfg.TLSConfig.CAPath = a.config.Nomad.CAPath
	}
	if a.config.Nomad.ClientCert != "" {
		cfg.TLSConfig.ClientCert = a.config.Nomad.ClientCert
	}
	if a.config.Nomad.ClientKey != "" {
		cfg.TLSConfig.ClientKey = a.config.Nomad.ClientKey
	}
	if a.config.Nomad.TLSServerName != "" {
		cfg.TLSConfig.TLSServerName = a.config.Nomad.TLSServerName
	}
	if a.config.Nomad.SkipVerify {
		cfg.TLSConfig.Insecure = a.config.Nomad.SkipVerify
	}

	// Generate the Nomad client.
	client, err := api.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to instantiate Nomad client: %v", err)
	}
	a.nomadClient = client

	return nil
}

// reload triggers the reload of sub-routines based on the operator sending a
// SIGHUP signal to the agent.
func (a Agent) reload() {
	a.policyManager.ReloadSources()
}

// handleSignals blocks until the agent receives an exit signal.
func (a *Agent) handleSignals() {

	signalCh := make(chan os.Signal, 3)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Wait to receive a signal. This blocks until we are notified.
WAIT:
	sig := <-signalCh

	a.logger.Info("caught signal", "signal", sig.String())

	// Check the signal we received. If it was a SIGHUP perform the reload
	// tasks and then continue to wait for another signal. Everything else
	// means exit.
	switch sig {
	case syscall.SIGHUP:
		a.reload()
		goto WAIT
	default:
		return
	}
}
