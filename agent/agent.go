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

type checkHandlerResult struct {
	action *strategypkg.Action
	err    error
	check  *policy.Check
}

type checkHandlerChannels struct {
	resultCh  <-chan checkHandlerResult
	proceedCh chan bool
}

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
			handlerCtx, cancel := context.WithTimeout(ctx, policyEval.Policy.EvaluationInterval)
			defer cancel()
			a.handlePolicy(handlerCtx, policyEval.Policy)
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

func (a *Agent) handlePolicy(ctx context.Context, p *policy.Policy) {
	checks := make(map[string]checkHandlerChannels)

	// Evaluate checks.
	for _, c := range p.Checks {
		resultCh := make(chan checkHandlerResult)
		defer close(resultCh)

		proceedCh := make(chan bool)
		defer close(proceedCh)

		channels := checkHandlerChannels{
			resultCh:  resultCh,
			proceedCh: proceedCh,
		}

		checks[c.Name] = channels

		go func(c *policy.Check) {
			a.handleCheck(ctx, p, c, resultCh, proceedCh)
		}(c)
	}

	// winningAction is the action to be executed after all checks' results are
	// reconciled.
	var winningAction *strategypkg.Action
	var winningCheck string

	for check, channels := range checks {
		logger := a.logger.With("check", check)

		select {
		case <-ctx.Done():
			logger.Warn("policy evaluation timeout")
			return
		case result := <-channels.resultCh:
			if result.err != nil {
				// TODO(luiz): properly handle errors.
				logger.Warn("failed to evaluate check: %v", result.err)
				continue
			}

			winningAction = strategypkg.PreemptAction(winningAction, result.action)
			if winningAction == result.action {
				winningCheck = result.check.Name
			}
		}
	}

	channels, ok := checks[winningCheck]
	if !ok {
		a.logger.Warn("invalid winning check %s", winningCheck)
		return
	}

	// Unblock winning check.
	channels.proceedCh <- true

	// Cancel all other checks.
	for _, ch := range checks {
		close(ch.proceedCh)
	}

	// Block until winning check is done or context is canceled.
	select {
	case <-ctx.Done():
		return
	case r := <-channels.resultCh:
		if r.err != nil {
			a.logger.Error("failed to execute check", "error", r.err, "check", r.check.Name)
		}
		return
	}
}

func (a *Agent) handleCheck(ctx context.Context, p *policy.Policy, c *policy.Check, resultCh chan<- checkHandlerResult, proceedCh <-chan bool) {
	logger := a.logger.With(
		"policy_id", p.ID,
		"target", p.Target.Name,
		"check", c.Name,
		"source", c.Source,
		"strategy", c.Strategy.Name,
	)

	logger.Info("received policy for evaluation")

	result := checkHandlerResult{check: c}

	var targetInst targetpkg.Target
	var apmInst apmpkg.APM
	var strategyInst strategypkg.Strategy

	// dispense plugins
	targetPlugin, err := a.pluginManager.Dispense(p.Target.Name, plugins.PluginTypeTarget)
	if err != nil {
		result.err = fmt.Errorf(`target plugin "%s" not initialized: %v`, p.Target.Name, err)
		resultCh <- result
		return
	}
	targetInst = targetPlugin.Plugin().(targetpkg.Target)

	apmPlugin, err := a.pluginManager.Dispense(c.Source, plugins.PluginTypeAPM)
	if err != nil {
		result.err = fmt.Errorf(`apm plugin "%s" not initialized: %v`, c.Source, err)
		resultCh <- result
		return
	}
	apmInst = apmPlugin.Plugin().(apmpkg.APM)

	strategyPlugin, err := a.pluginManager.Dispense(c.Strategy.Name, plugins.PluginTypeStrategy)
	if err != nil {
		result.err = fmt.Errorf(`strategy plugin "%s" not initialized: %v`, c.Strategy.Name, err)
		resultCh <- result
		return
	}
	strategyInst = strategyPlugin.Plugin().(strategypkg.Strategy)

	// fetch target count
	logger.Info("fetching current count")
	currentStatus, err := targetInst.Status(p.Target.Config)
	if err != nil {
		result.err = fmt.Errorf("failed to fetch current count: %v", err)
		resultCh <- result
		return
	}
	if !currentStatus.Ready {
		logger.Info("target not ready")
		result.action = &strategypkg.Action{Direction: strategypkg.ScaleDirectionDont}
		resultCh <- result
		return
	}

	// query policy's APM
	logger.Info("querying source", "query", c.Query)
	value, err := apmInst.Query(c.Query)
	if err != nil {
		result.err = fmt.Errorf("failed to query source: %v", err)
		resultCh <- result
		return
	}

	// calculate new count using policy's Strategy
	logger.Info("calculating new count")
	req := strategypkg.RunRequest{
		PolicyID: p.ID,
		Count:    currentStatus.Count,
		Metric:   value,
		Config:   c.Strategy.Config,
	}
	action, err := strategyInst.Run(req)
	if err != nil {
		result.err = fmt.Errorf("failed to execute strategy: %v", err)
		resultCh <- result
		return
	}

	if action.Direction == strategypkg.ScaleDirectionNone {
		// Make sure we are currently within [min, max] limits even if there's
		// no action to execute
		var minMaxAction *strategypkg.Action

		if currentStatus.Count < p.Min {
			minMaxAction = &strategypkg.Action{
				Count:     p.Min,
				Direction: strategypkg.ScaleDirectionUp,
				Reason:    fmt.Sprintf("current count (%d) below limit (%d)", currentStatus.Count, p.Min),
			}
		} else if currentStatus.Count > p.Max {
			minMaxAction = &strategypkg.Action{
				Count:     p.Max,
				Direction: strategypkg.ScaleDirectionDown,
				Reason:    fmt.Sprintf("current count (%d) above limit (%d)", currentStatus.Count, p.Max),
			}
		}

		if minMaxAction != nil {
			action = *minMaxAction
		} else {
			logger.Info("nothing to do")
			result.action = &strategypkg.Action{Direction: strategypkg.ScaleDirectionNone}
			resultCh <- result
			return
		}
	}

	// Make sure returned action has sane defaults instead of relying on
	// plugins doing this.
	action.Canonicalize()

	// Make sure new count value is within [min, max] limits
	action.CapCount(p.Min, p.Max)

	// Skip action if count doesn't change.
	if currentStatus.Count == action.Count {
		logger.Info("nothing to do", "from", currentStatus.Count, "to", action.Count)

		result.action = &strategypkg.Action{Direction: strategypkg.ScaleDirectionNone}
		resultCh <- result
		return
	}

	result.action = &action

	// Send result back and wait to see if we should proceed.
	resultCh <- result
	select {
	case <-ctx.Done():
		return
	case proceed := <-proceedCh:
		if !proceed {
			return
		}
	}

	// If the policy is configured with dry-run:true then we set the
	// action count to nil so its no-nop. This allows us to still
	// submit the job, but not alter its state.
	if val, ok := p.Target.Config["dry-run"]; ok && val == "true" {
		logger.Info("scaling dry-run is enabled, using no-op task group count")
		action.SetDryRun()
	}

	if action.Count == strategypkg.MetaValueDryRunCount {
		logger.Info("registering scaling event",
			"count", currentStatus.Count, "reason", action.Reason, "meta", action.Meta)
	} else {
		logger.Info("scaling target",
			"from", currentStatus.Count, "to", action.Count,
			"reason", action.Reason, "meta", action.Meta)
	}

	if err = targetInst.Scale(action, p.Target.Config); err != nil {
		logger.Error("failed to scale target", "error", err)
		// At this point nobody is listening on resultCh anymore.
		return
	}

	logger.Info("successfully submitted scaling action to target",
		"desired_count", action.Count)

	// Enforce the cooldown after a successful scaling event.
	a.policyManager.EnforceCooldown(p.ID, p.Cooldown)
}
