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

type Manager struct {
	lock            sync.RWMutex
	lockInternal    sync.RWMutex
	pluginClients   map[string]*plugin.Client
	internalPlugins map[string]*Strategy
}

func NewStrategyManager() *Manager {
	return &Manager{
		pluginClients:   make(map[string]*plugin.Client),
		internalPlugins: make(map[string]*Strategy),
	}
}

func (m *Manager) RegisterPlugin(key string, p *plugin.ClientConfig) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	client := plugin.NewClient(p)
	m.pluginClients[key] = client
	return nil
}

func (m *Manager) RegisterInternalPlugin(key string, p *Strategy) {
	m.lockInternal.Lock()
	defer m.lockInternal.Unlock()

	m.internalPlugins[key] = p
}

func (m *Manager) Dispense(key string) (*Strategy, error) {
	// check if this is a local implementation
	m.lockInternal.RLock()
	if s, ok := m.internalPlugins[key]; ok {
		m.lockInternal.RUnlock()
		return s, nil
	}
	m.lockInternal.RUnlock()

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
	strategy, ok := raw.(Strategy)
	if !ok {
		return nil, fmt.Errorf("plugins %s is not Strategy (%T)\n", key, raw)
	}

	return &strategy, nil
}

func (m *Manager) Kill() {
	for _, c := range m.pluginClients {
		c.Kill()
	}
}
