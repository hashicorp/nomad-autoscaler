package agent

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path"
	"sync"
	"time"

	"github.com/hashicorp/go-plugin"
	apmpkg "github.com/hashicorp/nomad-autoscaler/apm"
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
	config          *Config
	nomadClient     *api.Client
	apmPlugins      map[string]*Plugin
	apmManager      *apmpkg.Manager
	targetPlugins   map[string]*Plugin
	targetManager   *targetpkg.Manager
	strategyPlugins map[string]*Plugin
	strategyManager *strategypkg.Manager
}

type Plugin struct {
	client   *plugin.Client
	instance interface{}
}

func NewAgent(c *Config) *Agent {
	return &Agent{
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
	clientConfig := api.DefaultConfig()
	clientConfig = clientConfig.ClientConfig(a.config.Nomad.Region, a.config.Nomad.Address, false)

	client, err := api.NewClient(clientConfig)
	if err != nil {
		return fmt.Errorf("failed to instantiate Nomad client: %v", err)
	}

	a.nomadClient = client

	ps := policystorage.Nomad{Client: client}

	// launch plugins
	err = a.loadPlugins()
	if err != nil {
		return fmt.Errorf("failed to load plugins: %v", err)
	}

	// loop like there's no tomorrow
	var wg sync.WaitGroup
	interval, _ := time.ParseDuration(a.config.ScanInterval)
	ticker := time.NewTicker(interval)
Loop:
	for {
		select {
		case <-ticker.C:
			log.Printf("reading policies...\n")
			// read policies
			policies, err := ps.List()
			if err != nil {
				log.Printf("failed to fetch policies: %v", err)
				continue
			}

			// handle policies
			for _, p := range policies {
				policy, err := ps.Get(p.ID)
				if err != nil {
					log.Printf("failed to fetch policy %s: %v", p.ID, err)
					continue
				}

				wg.Add(1)
				go func(policy *policystorage.Policy) {
					defer wg.Done()
					select {
					case <-ctx.Done():
						return
					default:
						a.handlePolicy(policy)
					}
				}(policy)
			}
			wg.Wait()
		case <-ctx.Done():
			// stop plugins before exiting
			a.apmManager.Kill()
			a.targetManager.Kill()
			a.strategyManager.Kill()
			break Loop
		}
	}

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
	for _, apmConfig := range a.config.APMs {
		log.Printf("loading plugin: %v", apmConfig)

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
	a.config.Targets = append(a.config.Targets, Target{
		Name:   "local-nomad",
		Driver: "nomad",
		Config: map[string]string{
			"address": a.config.Nomad.Address,
			"region":  a.config.Nomad.Region,
		},
	})

	for _, targetConfig := range a.config.Targets {
		log.Printf("loading plugin: %v", targetConfig)

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
		log.Printf("loading plugin: %v", strategyConfig)

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
	var target targetpkg.Target
	var apm apmpkg.APM
	var strategy strategypkg.Strategy

	// dispense plugins
	targetPlugin, err := a.targetManager.Dispense(p.Target.Name)
	if err != nil {
		log.Printf("plugin %s not initialized: %v\n", p.Target.Name, err)
		return
	}
	target = *targetPlugin

	apmPlugin, err := a.apmManager.Dispense(p.Source)
	if err != nil {
		log.Printf("plugin %s not initialized: %v\n", p.Source, err)
		return
	}
	apm = *apmPlugin

	strategyPlugin, err := a.strategyManager.Dispense(p.Strategy.Name)
	if err != nil {
		log.Printf("plugin %s not initialized: %v\n", p.Strategy.Name, err)
		return
	}
	strategy = *strategyPlugin

	// fetch target count
	currentCount, err := target.Count(p.Target.Config)
	if err != nil {
		log.Printf("failed to fetch current count: %v", err)
		return
	}

	// query policy's APM
	value, err := apm.Query(p.Query)
	if err != nil {
		log.Printf("failed to query APM: %v\n", err)
		return
	}

	// calculate new count using policy's Strategy
	req := strategypkg.RunRequest{
		CurrentCount: currentCount,
		MinCount:     p.Strategy.Min,
		MaxCount:     p.Strategy.Max,
		CurrentValue: value,
		Config:       p.Strategy.Config,
	}
	results, err := strategy.Run(req)
	if err != nil {
		log.Printf("failed to calculate strategy: %v\n", err)
	}

	// scale target
	for _, action := range results.Actions {
		err = (*targetPlugin).Scale(action, p.Target.Config)
		if err != nil {
			log.Printf("failed to scale target: %v", err)
			return
		}
	}
}
