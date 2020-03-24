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
	Count  int64
	Reason string
	Meta   map[string]interface{}
}

func (a *Action) Canonicalize() {
	if a.Meta == nil {
		a.Meta = make(map[string]interface{})
	}
}

func (a *Action) IsWithinLimits(min, max int64) bool {
	return a.Count >= min && a.Count <= max
}

func (a *Action) CapCount(min, max int64) {
	if a.Count < min {
		a.Count = min
		a.PushReason(fmt.Sprintf("capping count to min value of %d", min))
	} else if a.Count > max {
		a.Count = max
		a.PushReason(fmt.Sprintf("capping count to max value of %d", max))
	}
}

func (a *Action) PushReason(r string) {
	metaKey := "nomad_autoscaler.reason_history"
	history := []string{}

	// Check if we already have a reason stack in Meta
	if historyInterface, ok := a.Meta[metaKey]; ok {
		if historySlice, ok := historyInterface.([]string); ok {
			history = historySlice
		}
	}

	// Append current reason to history and update action
	a.Meta[metaKey] = append(history, a.Reason)
	a.Reason = r
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
