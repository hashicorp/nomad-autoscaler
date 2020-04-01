package agent

import (
	"context"
	"fmt"
	"os/exec"
	"path"
	"reflect"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	apmpkg "github.com/hashicorp/nomad-autoscaler/apm"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/helper/plugins"
	"github.com/hashicorp/nomad-autoscaler/policystorage"
	strategypkg "github.com/hashicorp/nomad-autoscaler/strategy"
	targetpkg "github.com/hashicorp/nomad-autoscaler/target"
	"github.com/hashicorp/nomad/api"
)

var (
	PluginHandshakeConfig = plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "magic",
		MagicCookieValue: "magic",
	}
)

type Agent struct {
	logger          hclog.Logger
	config          *config.Agent
	nomadClient     *api.Client
	apmPlugins      map[string]*Plugin
	apmManager      *apmpkg.Manager
	targetPlugins   map[string]*Plugin
	targetManager   *targetpkg.Manager
	strategyPlugins map[string]*Plugin
	strategyManager *strategypkg.Manager
}

type Plugin struct{}

func NewAgent(c *config.Agent, logger hclog.Logger) *Agent {
	return &Agent{
		logger:          logger,
		config:          c,
		apmPlugins:      make(map[string]*Plugin),
		apmManager:      apmpkg.NewAPMManager(),
		targetPlugins:   make(map[string]*Plugin),
		targetManager:   targetpkg.NewTargetManager(),
		strategyPlugins: make(map[string]*Plugin),
		strategyManager: strategypkg.NewStrategyManager(),
	}
}

func (a *Agent) Run(ctx context.Context) error {

	// Generate the Nomad client.
	if err := a.generateNomadClient(); err != nil {
		return err
	}

	ps := policystorage.Nomad{Client: a.nomadClient}

	// launch plugins
	if err := a.loadPlugins(); err != nil {
		return fmt.Errorf("failed to load plugins: %v", err)
	}

	// Setup and start the HTTP health server.
	healthServer, err := newHealthServer(a.config.HTTP, a.logger)
	if err != nil {
		return fmt.Errorf("failed to setup HTTP getHealth server: %v", err)
	}
	go healthServer.run()

	// loop like there's no tomorrow
	var wg sync.WaitGroup
	ticker := time.NewTicker(a.config.ScanInterval)
Loop:
	for {
		select {
		case <-ticker.C:
			logger := a.logger.With("policy_storage", reflect.TypeOf(ps))
			logger.Info("reading policies")

			// read policies
			policies, err := ps.List()
			if err != nil {
				logger.Error("failed to fetch policies", "error", err)
				continue
			}
			logger.Info(fmt.Sprintf("found %d policies", len(policies)))

			// handle policies
			for _, p := range policies {
				wg.Add(1)
				go func(ID string) {
					defer wg.Done()
					select {
					case <-ctx.Done():
						return
					default:
						policy, err := ps.Get(ID)
						if err != nil {
							logger.Error("failed to fetch policy", "policy_id", ID, "error", err)
							return
						}
						a.handlePolicy(policy)
					}
				}(p.ID)
			}
			wg.Wait()
		case <-ctx.Done():
			// Stop the health server.
			healthServer.stop()

			// stop plugins before exiting
			a.logger.Info("killing plugins")
			a.apmManager.Kill()
			a.targetManager.Kill()
			a.strategyManager.Kill()
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

func (a *Agent) loadPlugins() error {
	// load APM plugins
	err := a.loadAPMPlugins()
	if err != nil {
		return err
	}

	// load target plugins
	err = a.loadTargetPlugins()
	if err != nil {
		return err
	}

	// load strategy plugins
	err = a.loadStrategyPlugins()
	if err != nil {
		return err
	}

	return nil
}

func (a *Agent) loadAPMPlugins() error {
	// create default local-nomad target
	a.config.APMs = append(a.config.APMs, &config.Plugin{
		Name:   "local-nomad",
		Driver: "nomad-apm",
		Config: nomadHelper.ConfigToMap(a.config.Nomad),
	})

	for _, apmConfig := range a.config.APMs {
		a.logger.Info("loading APM plugin", "plugin", apmConfig)

		if plugins.IsInternal(apmConfig.Driver, a.config.PluginDir) {
			plugin := plugins.NewInternalAPM(apmConfig.Driver)
			a.apmManager.RegisterInternalPlugin(apmConfig.Name, &plugin)

		} else {
			pluginConfig := &plugin.ClientConfig{
				HandshakeConfig: PluginHandshakeConfig,
				Plugins: map[string]plugin.Plugin{
					"apm": &apmpkg.Plugin{},
				},
				Cmd: exec.Command(path.Join(a.config.PluginDir, apmConfig.Driver)),
			}
			err := a.apmManager.RegisterPlugin(apmConfig.Name, pluginConfig)
			if err != nil {
				return err
			}
		}

		// configure plugin
		apmPlugin, err := a.apmManager.Dispense(apmConfig.Name)
		if err != nil {
			return err
		}
		err = (*apmPlugin).SetConfig(apmConfig.Config)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) loadTargetPlugins() error {
	// create default local-nomad target
	a.config.Targets = append(a.config.Targets, &config.Plugin{
		Name:   "local-nomad",
		Driver: "nomad",
		Config: nomadHelper.ConfigToMap(a.config.Nomad),
	})

	for _, targetConfig := range a.config.Targets {
		a.logger.Info("loading Target plugin", "plugin", targetConfig)

		if plugins.IsInternal(targetConfig.Driver, a.config.PluginDir) {
			plugin := plugins.NewInternalTarget(targetConfig.Driver)
			a.targetManager.RegisterInternalPlugin(targetConfig.Name, &plugin)

		} else {
			pluginConfig := &plugin.ClientConfig{
				HandshakeConfig: PluginHandshakeConfig,
				Plugins: map[string]plugin.Plugin{
					"target": &targetpkg.Plugin{},
				},
				Cmd: exec.Command(path.Join(a.config.PluginDir, targetConfig.Driver)),
			}
			err := a.targetManager.RegisterPlugin(targetConfig.Name, pluginConfig)
			if err != nil {
				return err
			}
		}

		// configure plugin
		targetPlugin, err := a.targetManager.Dispense(targetConfig.Name)
		if err != nil {
			return err
		}
		err = (*targetPlugin).SetConfig(targetConfig.Config)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) loadStrategyPlugins() error {
	for _, strategyConfig := range a.config.Strategies {
		a.logger.Info("loading Strategy plugin", "plugin", strategyConfig)

		if plugins.IsInternal(strategyConfig.Driver, a.config.PluginDir) {
			plugin := plugins.NewInternalStrategy(strategyConfig.Driver)
			a.strategyManager.RegisterInternalPlugin(strategyConfig.Name, &plugin)

		} else {
			pluginConfig := &plugin.ClientConfig{
				HandshakeConfig: PluginHandshakeConfig,
				Plugins: map[string]plugin.Plugin{
					"strategy": &strategypkg.Plugin{},
				},
				Cmd: exec.Command(path.Join(a.config.PluginDir, strategyConfig.Driver)),
			}
			err := a.strategyManager.RegisterPlugin(strategyConfig.Name, pluginConfig)
			if err != nil {
				return err
			}
		}

		// configure plugin
		strategyPlugin, err := a.strategyManager.Dispense(strategyConfig.Name)
		if err != nil {
			return err
		}
		err = (*strategyPlugin).SetConfig(strategyConfig.Config)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) handlePolicy(p *policystorage.Policy) {
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

	var target targetpkg.Target
	var apm apmpkg.APM
	var strategy strategypkg.Strategy

	// dispense plugins
	targetPlugin, err := a.targetManager.Dispense(p.Target.Name)
	if err != nil {
		logger.Error("target plugin not initialized", "error", err, "plugin", p.Target.Name)
		return
	}
	target = *targetPlugin

	apmPlugin, err := a.apmManager.Dispense(p.Source)
	if err != nil {
		logger.Error("apm plugin not initialized", "error", err, "plugin", p.Target.Name)
		return
	}
	apm = *apmPlugin

	strategyPlugin, err := a.strategyManager.Dispense(p.Strategy.Name)
	if err != nil {
		logger.Error("strategy plugin not initialized", "error", err, "plugin", p.Target.Name)
		return
	}
	strategy = *strategyPlugin

	// fetch target count
	logger.Info("fetching current count")
	currentCount, err := target.Count(p.Target.Config)
	if err != nil {
		logger.Error("failed to fetch current count", "error", err)
		return
	}

	// query policy's APM
	logger.Info("querying APM")
	value, err := apm.Query(p.Query)
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
	results, err := strategy.Run(req)
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
		// Make sure returned action has sane defaults instead of relying on
		// plugins doing this.
		action.Canonicalize()

		// If the policy is configured with dry-run:true then we set the
		// action count to nil so its no-nop. This allows us to still
		// submit the job, but not alter its state.
		if val, ok := p.Target.Config["dry-run"]; ok && val == "true" {
			logger.Info("scaling dry-run is enabled, using no-op task group count",
				"target_config", p.Target.Config)
			action.Meta["nomad_autoscaler.dry_run"] = "true"
			action.Meta["nomad_autoscaler.dry_run_count"] = action.Count
			action.Count = nil
		}

		// A nil count indicates that the current value shouldn't change,
		// so just notify the policy's target of this event.
		if action.Count == nil {
			logger.Info("no change in count")

			if err = (*targetPlugin).Scale(action, p.Target.Config); err != nil {
				logger.Error("failed to scale target", "error", err)
			}
			continue
		}

		// Check if this action will not violate the min and max limits set by
		// the policy.
		withinLimts, err := action.IsWithinLimits(p.Min, p.Max)
		if err != nil {
			logger.Error(fmt.Sprintf("failed to execute action: %s", err))
			continue
		}

		if !withinLimts {
			logger.Info("next count outside limits",
				"from", currentCount, "to", *action.Count, "min", p.Min, "max", p.Max)

			// Make sure new count value is within [min, max] limits
			err := action.CapCount(p.Min, p.Max)
			if err != nil {
				logger.Error(fmt.Sprintf("failed to execute action: %s", err))
				continue
			}

			logger.Info("updated count to be within limits",
				"from", currentCount, "to", *action.Count, "min", p.Min, "max", p.Max)
		}

		// If count is not nil, but its value doesn't change, assume that
		// no action should happen, not even an empty scale event.
		if *action.Count == currentCount {
			logger.Info("nothing to do: intended count equals current count",
				"from", currentCount, "to", *action.Count)
			continue
		}

		logger.Info("scaling target",
			"target_config", p.Target.Config, "from", currentCount, "to", *action.Count, "reason", action.Reason)

		if err = (*targetPlugin).Scale(action, p.Target.Config); err != nil {
			logger.Error("failed to scale target", "error", err)
			return
		}
	}
}
