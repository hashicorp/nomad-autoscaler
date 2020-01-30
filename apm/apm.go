package apm

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-plugin"
)

// APM interface plugins must implement
type APM interface {
	Query(q string) (float64, error)
	SetConfig(config map[string]string) error
}

type Manager struct {
	lock          sync.RWMutex
	pluginClients map[string]*plugin.Client
}

func NewAPMManager() *Manager {
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

func (m *Manager) Dispense(key string) (*APM, error) {
	var apm APM

	// check if this is a local implementation
	switch key {
	case "internal APM":
		// do something
	}
	if apm != nil {
		return &apm, nil
	}

	// otherwhise dispense a plugin
	m.lock.RLock()
	client := m.pluginClients[key]
	m.lock.RUnlock()

	rpcClient, err := client.Client()
	if err != nil {
		return nil, fmt.Errorf("failed to create RPC client: %v", err)
	}

	raw, err := rpcClient.Dispense("apm")
	if err != nil {
		return nil, fmt.Errorf("failed to dispense plugin: %v", err)
	}
	apm, ok := raw.(APM)
	if !ok {
		return nil, fmt.Errorf("plugins %s is not APM\n", key)
	}

	return &apm, nil
}

func (m *Manager) Kill() {
	for _, c := range m.pluginClients {
		c.Kill()
	}
}
