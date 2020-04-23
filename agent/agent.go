package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	apmpkg "github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	strategypkg "github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	targetpkg "github.com/hashicorp/nomad-autoscaler/plugins/target"
	newpolicy "github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/hashicorp/nomad-autoscaler/state"
	"github.com/hashicorp/nomad-autoscaler/state/policy"
	"github.com/hashicorp/nomad/api"
	"github.com/kr/pretty"
)

type Agent struct {
	logger        hclog.Logger
	config        *config.Agent
	nomadClient   *api.Client
	pluginManager *manager.PluginManager
}

func NewAgent(c *config.Agent, logger hclog.Logger) *Agent {
	return &Agent{
		logger: logger,
		config: c,
	}
}

func (a *Agent) Run(ctx context.Context) error {

	// Generate the Nomad client.
	if err := a.generateNomadClient(); err != nil {
		return err
	}

	stateHandlerConfig := &state.HandlerConfig{
		Logger:             a.logger,
		NomadClient:        a.nomadClient,
		EvaluationInterval: a.config.ScanInterval,
	}

	stateHandler := state.NewHandler(ctx, stateHandlerConfig)
	stateHandler.Start()

	// launch plugins
	if err := a.setupPlugins(); err != nil {
		return fmt.Errorf("failed to setup plugins: %v", err)
	}

	// Setup and start the HTTP health server.
	healthServer, err := newHealthServer(a.config.HTTP, a.logger)
	if err != nil {
		return fmt.Errorf("failed to setup HTTP getHealth server: %v", err)
	}
	go healthServer.run()

	source := newpolicy.NewNomadSource(a.logger, a.nomadClient)
	manager := newpolicy.NewManager(a.logger, source, a.pluginManager)

	policyEvalCh := make(chan *newpolicy.Evaluation, 10)
	go manager.Run(ctx, policyEvalCh)

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("done")
			return nil
		case policyEval := <-policyEvalCh:
			a.logger.Info("received policy eval", "policy_eval", pretty.Sprint(policyEval))
			//TODO(luiz): actually handle policy
		}
	}

	// loop like there's no tomorrow
	var wg sync.WaitGroup
	ticker := time.NewTicker(a.config.ScanInterval)
Loop:
	for {
		select {
		case <-ticker.C:

			// read policies
			policies := stateHandler.PolicyState.List()
			a.logger.Info(fmt.Sprintf("found %d policies", len(policies)))

			// handle policies
			for _, p := range policies {
				wg.Add(1)
				go func(policy *policy.Policy) {
					defer wg.Done()
					select {
					case <-ctx.Done():
						return
					default:
						a.handlePolicy(policy)
					}
				}(p)
			}
			wg.Wait()
		case <-ctx.Done():
			// Stop the health server.
			healthServer.stop()

			// stop plugins before exiting
			a.pluginManager.KillPlugins()
			break Loop
		}
	}

	return nil
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

	if !p.Enabled {
		logger.Info("policy not enabled")
		return
	}

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
	currentCount, err := targetInst.Count(p.Target.Config)
	if err != nil {
		logger.Error("failed to fetch current count", "error", err)
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
		Count:    currentCount,
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

		if currentCount < p.Min {
			minMaxAction = &strategypkg.Action{
				Count:  &p.Min,
				Reason: fmt.Sprintf("current count (%d) below limit (%d)", currentCount, p.Min),
			}
		} else if currentCount > p.Max {
			minMaxAction = &strategypkg.Action{
				Count:  &p.Max,
				Reason: fmt.Sprintf("current count (%d) above limit (%d)", currentCount, p.Max),
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
			action.SetDryRun(true)
		}

		if action.Count == nil {
			actionLogger.Info("registering scaling event",
				"count", currentCount, "reason", action.Reason, "meta", action.Meta)
		} else {
			actionLogger.Info("scaling target",
				"from", currentCount, "to", *action.Count,
				"reason", action.Reason, "meta", action.Meta)
		}

		if err = targetInst.Scale(action, p.Target.Config); err != nil {
			actionLogger.Error("failed to scale target", "error", err)
			continue
		}
	}
}
