package agent

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	apmpkg "github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	strategypkg "github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	targetpkg "github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/policy"
	nomadpolicy "github.com/hashicorp/nomad-autoscaler/policy/nomad"
	"github.com/hashicorp/nomad/api"
)

type Agent struct {
	logger        hclog.Logger
	config        *config.Agent
	nomadClient   *api.Client
	pluginManager *manager.PluginManager
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
		DefaultEvaluationInterval: a.config.DefaultEvaluationInterval,
	}
	source := nomadpolicy.NewNomadSource(a.logger, a.nomadClient, sourceConfig)
	manager := policy.NewManager(a.logger, source, a.pluginManager)

	policyEvalCh := make(chan *policy.Evaluation, 10)
	go manager.Run(ctx, policyEvalCh)

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("context closed, shutting down")
			return nil
		case policyEval := <-policyEvalCh:
			a.handlePolicy(policyEval.Policy)
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

func (a *Agent) handlePolicy(p *policy.Policy) {
	logger := a.logger.With(
		"policy_id", p.ID,
		"source", p.Source,
		"target", p.Target.Name,
		"strategy", p.Strategy.Name,
	)

	logger.Info("received policy for evaluation")

	var targetInst targetpkg.Target
	var apmInst apmpkg.APM
	var strategyInst strategypkg.Strategy

	// dispense plugins
	targetPlugin, err := a.pluginManager.Dispense(p.Target.Name, plugins.PluginTypeTarget)
	if err != nil {
		logger.Error("target plugin not initialized", "error", err, "plugin", p.Target.Name)
		return
	}
	targetInst = targetPlugin.Plugin().(targetpkg.Target)

	apmPlugin, err := a.pluginManager.Dispense(p.Source, plugins.PluginTypeAPM)
	if err != nil {
		logger.Error("apm plugin not initialized", "error", err, "plugin", p.Source)
		return
	}
	apmInst = apmPlugin.Plugin().(apmpkg.APM)

	strategyPlugin, err := a.pluginManager.Dispense(p.Strategy.Name, plugins.PluginTypeStrategy)
	if err != nil {
		logger.Error("strategy plugin not initialized", "error", err, "plugin", p.Strategy.Name)
		return
	}
	strategyInst = strategyPlugin.Plugin().(strategypkg.Strategy)

	// fetch target count
	logger.Info("fetching current count")
	currentStatus, err := targetInst.Status(p.Target.Config)
	if err != nil {
		logger.Error("failed to fetch current count", "error", err)
		return
	}
	if !currentStatus.Ready {
		logger.Info("target not ready")
		return
	}

	// query policy's APM
	logger.Info("querying APM")
	value, err := apmInst.Query(p.Query)
	if err != nil {
		logger.Error("failed to query APM", "error", err)
		return
	}

	// calculate new count using policy's Strategy
	logger.Info("calculating new count")
	req := strategypkg.RunRequest{
		PolicyID: p.ID,
		Count:    currentStatus.Count,
		Metric:   value,
		Config:   p.Strategy.Config,
	}
	results, err := strategyInst.Run(req)
	if err != nil {
		logger.Error("failed to calculate strategy", "error", err)
		return
	}

	if len(results.Actions) == 0 {
		// Make sure we are currently within [min, max] limits even if there's
		// no action to execute
		var minMaxAction *strategypkg.Action

		if currentStatus.Count < p.Min {
			minMaxAction = &strategypkg.Action{
				Count:  p.Min,
				Reason: fmt.Sprintf("current count (%d) below limit (%d)", currentStatus.Count, p.Min),
			}
		} else if currentStatus.Count > p.Max {
			minMaxAction = &strategypkg.Action{
				Count:  p.Max,
				Reason: fmt.Sprintf("current count (%d) above limit (%d)", currentStatus.Count, p.Max),
			}
		}

		if minMaxAction != nil {
			results.Actions = append(results.Actions, *minMaxAction)
		} else {
			logger.Info("nothing to do")
			return
		}
	}

	// scale target
	for _, action := range results.Actions {
		actionLogger := logger.With("target_config", p.Target.Config)

		// Make sure returned action has sane defaults instead of relying on
		// plugins doing this.
		action.Canonicalize()

		// Make sure new count value is within [min, max] limits
		action.CapCount(p.Min, p.Max)

		// If the policy is configured with dry-run:true then we set the
		// action count to nil so its no-nop. This allows us to still
		// submit the job, but not alter its state.
		if val, ok := p.Target.Config["dry-run"]; ok && val == "true" {
			actionLogger.Info("scaling dry-run is enabled, using no-op task group count")
			action.SetDryRun()
		}

		if action.Count == strategypkg.MetaValueDryRunCount {
			actionLogger.Info("registering scaling event",
				"count", currentStatus.Count, "reason", action.Reason, "meta", action.Meta)
		} else {
			// Skip action if count doesn't change.
			if currentStatus.Count == action.Count {
				actionLogger.Info("nothing to do", "from", currentStatus.Count, "to", action.Count)
				continue
			}

			actionLogger.Info("scaling target",
				"from", currentStatus.Count, "to", action.Count,
				"reason", action.Reason, "meta", action.Meta)
		}

		if err = targetInst.Scale(action, p.Target.Config); err != nil {
			actionLogger.Error("failed to scale target", "error", err)
			continue
		}
	}
}
