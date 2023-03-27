// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	metrics "github.com/armon/go-metrics"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	"github.com/hashicorp/nomad-autoscaler/policy"
	filePolicy "github.com/hashicorp/nomad-autoscaler/policy/file"
	nomadPolicy "github.com/hashicorp/nomad-autoscaler/policy/nomad"
	"github.com/hashicorp/nomad-autoscaler/policyeval"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/nomad"
	"github.com/hashicorp/nomad/api"
)

type Agent struct {
	logger        hclog.Logger
	config        *config.Agent
	configPaths   []string
	nomadClient   *api.Client
	consulClient  *consulapi.Client
	pluginManager *manager.PluginManager
	policySources map[policy.SourceName]policy.Source
	policyManager *policy.Manager
	inMemSink     *metrics.InmemSink
	evalBroker    *policyeval.Broker

	// nomadCfg is the merged Nomad API configuration that should be used when
	// setting up all clients. It is the result of the Nomad api.DefaultConfig
	// merged with the user-specified Nomad config.Nomad.
	nomadCfg *api.Config
}

func NewAgent(c *config.Agent, configPaths []string, logger hclog.Logger) *Agent {
	return &Agent{
		logger:      logger,
		config:      c,
		configPaths: configPaths,
		nomadCfg:    nomadHelper.MergeDefaultWithAgentConfig(c.Nomad),
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

	// Generate the Consul client
	if err := a.generateConsulClient(); err != nil {
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
	a.inMemSink = inMem

	// Setup policy manager.
	policyEvalCh, err := a.setupPolicyManager()
	if err != nil {
		return fmt.Errorf("failed to setup policy manager: %v", err)
	}
	go a.policyManager.Run(ctx, policyEvalCh)

	// Launch eval broker and workers.
	a.evalBroker = policyeval.NewBroker(
		a.logger.ResetNamed("policy_eval"),
		a.config.PolicyEval.AckTimeout,
		a.config.PolicyEval.DeliveryLimit)
	a.initWorkers(ctx)

	a.initEnt(ctx)

	// Launch the eval handler.
	go a.runEvalHandler(ctx, policyEvalCh)

	// Wait for our exit.
	a.handleSignals()
	return nil
}

func (a *Agent) runEvalHandler(ctx context.Context, evalCh chan *sdk.ScalingEvaluation) {
	for {
		select {
		case <-ctx.Done():
			a.logger.Info("context closed, shutting down eval handler")
			return
		case policyEval := <-evalCh:
			a.evalBroker.Enqueue(policyEval)
		}
	}
}

func (a *Agent) initWorkers(ctx context.Context) {
	policyEvalLogger := a.logger.ResetNamed("policy_eval")

	workersCount := []interface{}{}
	for k, v := range a.config.PolicyEval.Workers {
		workersCount = append(workersCount, k, v)
	}
	policyEvalLogger.Info("starting workers", workersCount...)

	for i := 0; i < a.config.PolicyEval.Workers["horizontal"]; i++ {
		w := policyeval.NewBaseWorker(
			policyEvalLogger, a.pluginManager, a.policyManager, a.evalBroker, "horizontal")
		go w.Run(ctx)
	}

	for i := 0; i < a.config.PolicyEval.Workers["cluster"]; i++ {
		w := policyeval.NewBaseWorker(
			policyEvalLogger, a.pluginManager, a.policyManager, a.evalBroker, "cluster")
		go w.Run(ctx)
	}
}

func (a *Agent) setupPolicyManager() (chan *sdk.ScalingEvaluation, error) {

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
			sources[policy.SourceNameNomad] = nomadPolicy.NewNomadSource(a.logger, a.nomadClient, policyProcessor)
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
		return nil, fmt.Errorf("no policy source available")
	}

	a.policySources = sources
	a.policyManager = policy.NewManager(a.logger, a.policySources, a.pluginManager, a.config.Telemetry.CollectionInterval)

	return make(chan *sdk.ScalingEvaluation, 10), nil
}

func (a *Agent) stop() {
	// Kill all the plugins.
	if a.pluginManager != nil {
		a.pluginManager.KillPlugins()
	}
}

// generateNomadClient creates a Nomad client for use within the agent.
func (a *Agent) generateNomadClient() error {

	// Generate the Nomad client.
	client, err := api.NewClient(a.nomadCfg)
	if err != nil {
		return fmt.Errorf("failed to instantiate Nomad client: %v", err)
	}
	a.nomadClient = client

	return nil
}

// generateConsulClient creates a Consul client for use within the agent.
func (a *Agent) generateConsulClient() error {
	if a.config.Consul == nil {
		a.consulClient = nil
		return nil
	}

	cfg, err := a.config.Consul.MergeWithDefault()
	if err != nil {
		return fmt.Errorf("error generating Consul client config: %v", err)
	}

	// Generate the Consul client.
	client, err := consulapi.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to instantiate Consul client: %v", err)
	}
	a.consulClient = client

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

	if err := a.generateNomadClient(); err != nil {
		a.logger.Error("failed to reload Autoscaler configuration", "error", err)
		os.Exit(1)
	}

	if err := a.generateConsulClient(); err != nil {
		a.logger.Error("failed to reload Autoscaler configuration", "error", err)
		os.Exit(1)
	}

	a.logger.Debug("reloading policy sources")
	// Set new Nomad client in the Nomad policy source.
	ps, ok := a.policySources[policy.SourceNameNomad]
	if ok {
		ps.(*nomadPolicy.Source).SetNomadClient(a.nomadClient)
	}
	a.policyManager.ReloadSources()

	a.logger.Debug("reloading plugins")
	if err := a.pluginManager.Reload(a.setupPluginsConfig()); err != nil {
		a.logger.Error("failed to reload plugins", "error", err)
	}
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
