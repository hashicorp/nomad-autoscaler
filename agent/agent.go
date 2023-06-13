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
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/hashicorp/nomad-autoscaler/policyeval"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/source"
	"github.com/hashicorp/nomad-autoscaler/source/nomad"
	"github.com/hashicorp/nomad/api"
)

type Agent struct {
	logger        hclog.Logger
	config        *config.Agent
	configPaths   []string
	nomadClient   *api.Client
	pluginManager *manager.PluginManager
	policySources map[source.Name]source.Source
	policyManager *policy.Manager
	inMemSink     *metrics.InmemSink
	evalBroker    *policyeval.Broker
	policyEvalCh  chan *sdk.ScalingEvaluation

	// nomadCfg is the merged Nomad API configuration that should be used when
	// setting up all clients. It is the result of the Nomad api.DefaultConfig
	// merged with the user-specified Nomad config.Nomad.
	nomadCfg *api.Config
}

func NewAgent(c *config.Agent, configPaths []string, logger hclog.Logger,
	pluginm *manager.PluginManager, policym *policy.Manager, nomadClient *api.Client,
	policyEvalCh chan *sdk.ScalingEvaluation) *Agent {
	return &Agent{
		logger:        logger,
		config:        c,
		configPaths:   configPaths,
		nomadCfg:      nomadHelper.MergeDefaultWithAgentConfig(c.Nomad),
		policyManager: policym,
		pluginManager: pluginm,
		//nomadClient:   nomadClient,
		policyEvalCh: policyEvalCh,
	}
}

func (a *Agent) Run(ctx context.Context) error {
	//defer a.stop()

	// Setup the telemetry sinks.
	inMem, err := a.setupTelemetry(a.config.Telemetry)
	if err != nil {
		return fmt.Errorf("failed to setup telemetry: %v", err)
	}
	a.inMemSink = inMem

	// Launch eval broker and workers.
	a.evalBroker = policyeval.NewBroker(
		a.logger.ResetNamed("policy_eval"),
		a.config.PolicyEval.AckTimeout,
		a.config.PolicyEval.DeliveryLimit)
	a.initWorkers(ctx)

	a.initEnt(ctx)

	// Launch the eval handler.
	go a.runEvalHandler(ctx, a.policyEvalCh)

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

	a.logger.Debug("reloading policy sources")
	// Set new Nomad client in the Nomad policy source.
	ps, ok := a.policySources[source.NameNomad]
	if ok {
		ps.(*nomad.NomadSource).SetNomadClient(a.nomadClient)
	}
	a.policyManager.ReloadSources()

	/*
		 	a.logger.Debug("reloading plugins")
			if err := a.pluginManager.Reload(a.setupPluginsConfig()); err != nil {
				a.logger.Error("failed to reload plugins", "error", err)
			}
	*/
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
