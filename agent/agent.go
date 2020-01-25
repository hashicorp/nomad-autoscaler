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
	"github.com/hashicorp/nomad-autoscaler/apm"
	"github.com/hashicorp/nomad-autoscaler/nomad/api"
	nomadApi "github.com/hashicorp/nomad-autoscaler/nomad/api"
	"github.com/hashicorp/nomad-autoscaler/strategy"
	targetstrategy "github.com/hashicorp/nomad-autoscaler/strategy/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/target"
)

var (
	PluginHandshakeConfig = plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "magic",
		MagicCookieValue: "magic",
	}
)

type Agent struct {
	config      *Config
	nomadClient *api.Client
	apmPlugins  map[string]*Plugin
	apmManager  *apm.Manager
}

type Plugin struct {
	client   *plugin.Client
	instance interface{}
}

func NewAgent(c *Config) *Agent {
	return &Agent{
		config:     c,
		apmPlugins: make(map[string]*Plugin),
		apmManager: apm.NewAPMManager(),
	}
}

func (a *Agent) Run(ctx context.Context) error {
	client, err := nomadApi.NewClient(nil)
	if err != nil {
		return fmt.Errorf("failed to create Nomad API client: %v", err)
	}

	a.nomadClient = client

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
		case <-ctx.Done():
			// stop plugins before exiting
			a.apmManager.Kill()
			break Loop
		case <-ticker.C:
			// read policies
			policies, err := a.nomadClient.Policies().List()
			if err != nil {
				log.Printf("failed to fetch policies: %v", err)
				continue
			}

			// handle policies
			for _, p := range policies {
				wg.Add(1)
				go func(p *api.PolicyList) {
					defer wg.Done()
					select {
					case <-ctx.Done():
						return
					default:
						a.handlePolicy(p)
					}
				}(p)
			}
			wg.Wait()
		}
	}

	return nil
}

func (a *Agent) loadPlugins() error {
	for _, apmConfig := range a.config.APMs {
		log.Printf("loading plugin: %v", apmConfig)

		pluginConfig := &plugin.ClientConfig{
			HandshakeConfig: PluginHandshakeConfig,
			Plugins: map[string]plugin.Plugin{
				"apm": &apm.Plugin{},
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

func (a *Agent) handlePolicy(p *api.PolicyList) {
	// fetch target count
	var t target.Target
	switch p.Target.Name {
	case "nomad_group_count":
		t = &target.NomadGroupCount{}
	}

	currentCount, err := t.Count(p.Target.Config)
	if err != nil {
		log.Printf("failed to fetch current count: %v", err)
		return
	}

	// query policy's APM
	apmPlugin, err := a.apmManager.Dispense(p.Source)
	if err != nil {
		log.Printf("plugin %s not initialized: %v\n", p.Source, err)
		return
	}

	value, err := (*apmPlugin).Query(p.Query)
	if err != nil {
		log.Printf("failed to query APM: %v\n", err)
		return
	}

	// calculate new count using policy's Strategy
	var s strategy.Strategy
	switch p.Strategy.Name {
	case "target":
		s = &targetstrategy.TargetStrategy{}
	}

	if s == nil {
		log.Printf("strategy %s not valid", p.Strategy.Name)
		return
	}

	req := &strategy.RunRequest{
		CurrentCount: currentCount,
		MinCount:     p.Strategy.Min,
		MaxCount:     p.Strategy.Max,
		CurrentValue: value,
		Config:       p.Strategy.Config,
	}
	results, err := s.Run(req)
	if err != nil {
		log.Printf("failed to calculate strategy: %v\n", err)
	}

	// scale target
	err = t.Scale(results, p.Target.Config)
	if err != nil {
		log.Printf("failed to scale target: %v", err)
		return
	}
}
