package target

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/strategy"
)

type Target interface {
	SetConfig(config map[string]string) error
	Count(config map[string]string) (int, error)
	Scale(action strategy.Action, config map[string]string) error
}

type Manager struct {
	lock          sync.RWMutex
	pluginClients map[string]*plugin.Client
}

func NewTargetManager() *Manager {
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

func (m *Manager) Dispense(key string) (*Target, error) {
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

	raw, err := rpcClient.Dispense("target")
	if err != nil {
		return nil, fmt.Errorf("failed to dispense plugin: %v", err)
	}
	target, ok := raw.(Target)
	if !ok {
		return nil, fmt.Errorf("plugins %s is not Target\n", key)
	}

	return &target, nil
}

func (m *Manager) Kill() {
	for _, c := range m.pluginClients {
		c.Kill()
	}
}
