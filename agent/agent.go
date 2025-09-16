// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hashicorp/go-hclog"
	metrics "github.com/hashicorp/go-metrics"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	"github.com/hashicorp/nomad-autoscaler/policy"
	filePolicy "github.com/hashicorp/nomad-autoscaler/policy/file"
	nomadPolicy "github.com/hashicorp/nomad-autoscaler/policy/nomad"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/nomad"
	"github.com/hashicorp/nomad/api"
)

type Agent struct {
	NomadClient *api.Client

	logger        hclog.Logger
	config        *config.Agent
	configPaths   []string
	pluginManager *manager.PluginManager
	policySources map[policy.SourceName]policy.Source
	policyManager *policy.Manager
	inMemSink     *metrics.InmemSink

	// nomadCfg is the merged Nomad API configuration that should be used when
	// setting up all clients. It is the result of the Nomad api.DefaultConfig
	// merged with the user-specified Nomad config.Nomad.
	nomadCfg *api.Config

	// entReload is used to notify the Enterprise license watcher to reload its
	// configuration.
	entReload chan any
}

func NewAgent(c *config.Agent, configPaths []string, logger hclog.Logger) *Agent {
	return &Agent{
		logger:      logger,
		config:      c,
		configPaths: configPaths,
		nomadCfg:    nomadHelper.MergeDefaultWithAgentConfig(c.Nomad),
		entReload:   make(chan any),
	}
}

func (a *Agent) Run(ctx context.Context) error {
	defer a.stop()

	// launch plugins
	if err := a.setupPlugins(); err != nil {
		return fmt.Errorf("failed to setup plugins: %v", err)
	}

	// Setup the telemetry sinks.
	inMem, err := a.setupTelemetry(a.config.Telemetry)
	if err != nil {
		return fmt.Errorf("failed to setup telemetry: %v", err)
	}
	a.inMemSink = inMem

	// Setup policy manager.
	policyEvalCh := make(chan *sdk.ScalingEvaluation, 10)
	defer close(policyEvalCh)

	limiter := policy.NewLimiter(policy.DefaultLimiterTimeout,
		a.config.PolicyEval.Workers)

	if err := a.setupPolicyManager(limiter); err != nil {
		return fmt.Errorf("failed to setup policy manager: %v", err)
	}

	go a.policyManager.Run(ctx, policyEvalCh)

	a.initEnt(ctx, a.entReload)

	// Wait for our exit.
	a.handleSignals()
	return nil
}

func (a *Agent) stop() {
	// Kill all the plugins.
	if a.pluginManager != nil {
		a.pluginManager.KillPlugins()
	}
}

// GenerateNomadClient creates a Nomad client for use within the agent.
func (a *Agent) GenerateNomadClient() error {

	// Generate the Nomad client.
	client, err := api.NewClient(a.nomadCfg)
	if err != nil {
		return fmt.Errorf("failed to instantiate Nomad client: %v", err)
	}
	a.NomadClient = client

	return nil
}

// reload triggers the reload of sub-routines based on the operator sending a
// SIGHUP signal to the agent.
func (a *Agent) reload() {
	a.logger.Info("reloading Autoscaler configuration")

	// Reload config files from disk.
	// Exit on error so operators can detect and correct configuration early.
	// TODO: revisit this once we have a better mechanism for surfacing errors.
	newCfg, err := config.LoadPaths(a.configPaths)
	if err != nil {
		a.logger.Error("failed to reload Autoscaler configuration", "error", err)
		os.Exit(1)
	}

	a.config = newCfg
	a.nomadCfg = nomadHelper.MergeDefaultWithAgentConfig(newCfg.Nomad)

	if err := a.GenerateNomadClient(); err != nil {
		a.logger.Error("failed to reload Autoscaler configuration", "error", err)
		os.Exit(1)
	}

	a.entReload <- struct{}{}

	a.logger.Debug("reloading policy sources")
	// Set new Nomad client in the Nomad policy source.
	ps, ok := a.policySources[policy.SourceNameNomad]
	if ok {
		ps.(*nomadPolicy.Source).SetNomadClient(a.NomadClient)
	}
	a.policyManager.ReloadSources()

	a.logger.Debug("reloading plugins")
	if err := a.pluginManager.Reload(a.setupPluginsConfig()); err != nil {
		a.logger.Error("failed to reload plugins", "error", err)
	}
}

func (a *Agent) setupPolicyManager(limiter *policy.Limiter) error {

	// Create our processor, a shared method for performing basic policy
	// actions.
	cfgDefaults := policy.ConfigDefaults{
		DefaultEvaluationInterval: a.config.Policy.DefaultEvaluationInterval,
		DefaultCooldown:           a.config.Policy.DefaultCooldown,
	}
	policyProcessor := policy.NewProcessor(&cfgDefaults, a.getNomadAPMNames())

	// Setup our initial default policy source which is Nomad.
	sources := map[policy.SourceName]policy.Source{}
	for _, s := range a.config.Policy.Sources {
		if s.Enabled == nil || !*s.Enabled {
			continue
		}

		switch policy.SourceName(s.Name) {
		case policy.SourceNameNomad:
			sources[policy.SourceNameNomad] = nomadPolicy.NewNomadSource(a.logger, a.NomadClient, policyProcessor)
		case policy.SourceNameFile:
			// Only setup the file source if operators have configured a
			// scaling policy directory to read from.
			if a.config.Policy.Dir != "" {
				sources[policy.SourceNameFile] = filePolicy.NewFileSource(a.logger, a.config.Policy.Dir, policyProcessor)
			}
		}
	}

	// TODO: Once full policy source reload is implemented this should probably
	// be just a warning.
	if len(sources) == 0 {
		return errors.New("no policy source available")
	}

	a.policySources = sources
	a.policyManager = policy.NewManager(a.logger, a.policySources,
		a.pluginManager, a.config.Telemetry.CollectionInterval, limiter)

	return nil
}

// handleSignals blocks until the agent receives an exit signal.
func (a *Agent) handleSignals() {

	signalCh := make(chan os.Signal, 3)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Wait to receive a signal. This blocks until we are notified.
	for {
		sig := <-signalCh

		a.logger.Info("caught signal", "signal", sig.String())

		// Check the signal we received. If it was a SIGHUP perform the reload
		// tasks and then continue to wait for another signal. Everything else
		// means exit.
		switch sig {
		case syscall.SIGHUP:
			a.reload()
		default:
			return
		}
	}
}
