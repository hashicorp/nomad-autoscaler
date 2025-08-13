// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package agent

import (
	"context"
	"errors"

	"github.com/hashicorp/nomad-autoscaler/policy"
	filePolicy "github.com/hashicorp/nomad-autoscaler/policy/file"
	nomadPolicy "github.com/hashicorp/nomad-autoscaler/policy/nomad"
)

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

func (a *Agent) initEnt(ctx context.Context, reload <-chan any) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-reload:
				continue
			}
		}
	}()
}
