package strategy

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-plugin"
)

type Strategy interface {
	SetConfig(config map[string]string) error
	Run(req RunRequest) (RunResponse, error)
}

type Action struct {
	Count  int
	Reason string
}

type Manager struct {
	lock          sync.RWMutex
	pluginClients map[string]*plugin.Client
}

func NewStrategyManager() *Manager {
	return &Manager{
		pluginClients: make(map[string]*plugin.Client),
	}
}

func (m *Manager) RegisterPlugin(key string, p *plugin.ClientConfig) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	client := plugin.NewClient(p)
	m.pluginClients[key] = client
	return nil
}

func (m *Manager) Dispense(key string) (*Strategy, error) {
	m.lock.RLock()
	client := m.pluginClients[key]
	m.lock.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("missing client %s", key)
	}

	rpcClient, err := client.Client()
	if err != nil {
		return nil, fmt.Errorf("failed to create RPC client: %v", err)
	}

	raw, err := rpcClient.Dispense("strategy")
	if err != nil {
		return nil, fmt.Errorf("failed to dispense plugin: %v", err)
	}
	target, ok := raw.(Strategy)
	if !ok {
		return nil, fmt.Errorf("plugins %s is not Strategy (%T)\n", key, raw)
	}

	return &target, nil
}

func (m *Manager) Kill() {
	for _, c := range m.pluginClients {
		c.Kill()
	}
}
