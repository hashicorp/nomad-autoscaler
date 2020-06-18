package agent

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	"github.com/hashicorp/nomad-autoscaler/policy"
	nomadpolicy "github.com/hashicorp/nomad-autoscaler/policy/nomad"
	"github.com/hashicorp/nomad/api"
)

type Agent struct {
	logger        hclog.Logger
	config        *config.Agent
	nomadClient   *api.Client
	pluginManager *manager.PluginManager
	policyManager *policy.Manager
	healthServer  *healthServer
}

func NewAgent(c *config.Agent, logger hclog.Logger) *Agent {
	return &Agent{
		logger: logger,
		config: c,
	}
}

func (a *Agent) Run(ctx context.Context) error {
	defer a.stop()

	// Generate the Nomad client.
	if err := a.generateNomadClient(); err != nil {
		return err
	}

	// launch plugins
	if err := a.setupPlugins(); err != nil {
		return fmt.Errorf("failed to setup plugins: %v", err)
	}

	// Setup and start the HTTP health server.
	healthServer, err := newHealthServer(a.config.HTTP, a.logger)
	if err != nil {
		return fmt.Errorf("failed to setup HTTP getHealth server: %v", err)
	}

	a.healthServer = healthServer
	go a.healthServer.run()

	sourceConfig := &nomadpolicy.SourceConfig{
		DefaultCooldown:           a.config.Policy.DefaultCooldown,
		DefaultEvaluationInterval: a.config.DefaultEvaluationInterval,
	}
	source := nomadpolicy.NewNomadSource(a.logger, a.nomadClient, sourceConfig)
	a.policyManager = policy.NewManager(a.logger, source, a.pluginManager)

	policyEvalCh := make(chan *policy.Evaluation, 10)
	go a.policyManager.Run(ctx, policyEvalCh)

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("context closed, shutting down")
			return nil
		case policyEval := <-policyEvalCh:
			w := policy.NewWorker(a.logger, a.pluginManager, a.policyManager)
			go w.HandlePolicy(ctx, policyEval.Policy)
		}
	}
}

func (a *Agent) stop() {
	// Stop the health server.
	if a.healthServer != nil {
		a.healthServer.stop()
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
