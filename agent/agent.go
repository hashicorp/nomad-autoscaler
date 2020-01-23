package agent

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/apm"
	"github.com/hashicorp/nomad-autoscaler/nomad/api"
	nomadApi "github.com/hashicorp/nomad-autoscaler/nomad/api"
	"github.com/hashicorp/nomad-autoscaler/strategy"
	targetstrategy "github.com/hashicorp/nomad-autoscaler/strategy/plugins/target"
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

func (a *Agent) Run() error {
	// TODO: handle signals properly
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		// kill plugis on exit
		a.apmManager.Kill()
		os.Exit(0)
	}()

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
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		// read policies
		policies, err := client.Policies().List()
		if err != nil {
			log.Printf("failed to fetch policies: %v", err)
			continue
		}

		// handle policies
		var wg sync.WaitGroup
		for _, p := range policies {
			wg.Add(1)
			go func(p *api.PolicyList) {
				defer wg.Done()
				a.handlePolicy(p)
			}(p)
		}
		wg.Wait()
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
		(*apmPlugin).SetConfig(apmConfig.Config)
	}
	return nil
}

func (a *Agent) handlePolicy(p *api.PolicyList) {
	// fetch job

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
	log.Println(value)

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

	results, err := s.Run(p.Strategy.Config)
	if err != nil {
		log.Printf("failed to calculate strategy: %v\n", err)
	}

	// update count in Nomad
	for _, result := range results {
		req := nomadApi.JobScaleRequest{
			JobID:  result.JobID,
			Count:  result.Count,
			Reason: result.Reason,
		}
		a.nomadClient.Jobs().Scale(req)
	}
}
