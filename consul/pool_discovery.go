package consul

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad-autoscaler/ha"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/hash"
)

// AgentID satisfies the AgentID function on the PoolDiscovery interface.
func (c *Consul) AgentID() string { return c.agentID }

// MonitorPool satisfies the MonitorPool function on the PoolDiscovery
// interface.
func (c *Consul) MonitorPool(ctx context.Context, req ha.DiscoveryRequest) {
	q := &api.QueryOptions{
		AllowStale: true,
		WaitIndex:  0,
		WaitTime:   5 * time.Minute,
	}
	for {
		errCh := make(chan error)
		resCh := make(chan []*api.CatalogService)
		go func() {
			c.clientLock.RLock()
			defer c.clientLock.RUnlock()

			services, qm, err := c.client.Catalog().Service(c.serviceName, autoscalerConsulServiceTag, q)
			if err != nil {
				errCh <- err
			} else {
				q.WaitIndex = qm.LastIndex
				resCh <- services
			}
		}()
		select {
		case <-ctx.Done():
			return
		case <-c.poolDiscoveryReloadCh:
			continue
		case err := <-errCh:
			req.ErrCh <- fmt.Errorf("error querying Consul catalog: %v", err)
			time.Sleep(5 * time.Second) // maybe a backoff here?
		case services := <-resCh:
			pool := make([]hash.RingMember, len(services))
			for i, service := range services {
				pool[i] = hash.NewRingMember(service.ServiceID)
			}
			req.UpdateCh <- pool
		}
	}
}

// RegisterAgent satisfies the RegisterAgent function on the PoolDiscovery
// interface.
func (c *Consul) RegisterAgent(ctx context.Context) error {

	agentID, err := ha.GenerateAgentID()
	if err != nil {
		return err
	}

	// Set important information used when registering this agent.
	c.agentID = agentID
	c.serviceID = fmt.Sprintf("_%s_%s", c.serviceName, agentID)

	// TODO (jrasell) we should add basic retry.
	if err := c.registerAgent(); err != nil {
		return err
	}

	// Start a goroutine to deregister me on context cancellation. This is best
	// effort only. We should not remove the ID file if it exists. When running
	// on metal, shutdown of the autoscaler could be due to maintenance and we
	// want to come back up with the same ID; like how Nomad does.
	go func() {
		<-ctx.Done()
		_ = c.deregisterAgent()
		c.poolDiscoveryDoneCh <- struct{}{}
	}()

	return nil
}

// Reload satisfies the Reload function on the PoolDiscovery interface.
func (c *Consul) Reload() { c.poolDiscoveryReloadCh <- struct{}{} }

// WaitForExit satisfies the WaitForExit function on the PoolDiscovery
// interface.
func (c *Consul) WaitForExit() {
	select {
	case <-time.After(shutdownTimeout):
	case <-c.poolDiscoveryDoneCh:
	}
}

// registerAgent performs the Autoscaler agent registration within Consul along
// with a check. This check is a basic TCP check targeting the HTTP API.
func (c *Consul) registerAgent() error {

	// Generate a service registration for this agent. The meta provides an
	// insight to operators as to where it came from, but as yet we don't have
	// the flashy label Nomad has.
	serviceReg := api.AgentServiceRegistration{
		ID:      c.agentID,
		Name:    "nomad-autoscaler",
		Tags:    []string{autoscalerConsulServiceTag},
		Address: c.agentAddr,
		Port:    c.agentPort,
		Meta:    map[string]string{"external-source": "nomad-autoscaler"},
	}

	// Generate a check registration object, ensuring the ID is unique.
	checkReg := api.AgentCheckRegistration{
		Name:      "nomad-autoscaler-agent",
		ServiceID: serviceReg.ID,
	}
	checkReg.TCP = net.JoinHostPort(c.agentAddr, strconv.Itoa(c.agentPort))
	checkReg.Interval = "10s"
	checkReg.ID = generateCheckID(checkReg, c.agentID)

	c.clientLock.RLock()
	defer c.clientLock.RUnlock()

	// Register and format the errors to provide clearer feedback.
	if err := c.client.Agent().ServiceRegister(&serviceReg); err != nil {
		return fmt.Errorf("failed to register service: %v", err)
	}
	if err := c.client.Agent().CheckRegister(&checkReg); err != nil {
		return fmt.Errorf("failed to register check: %v", err)
	}
	return nil
}

// DeregisterAgent satisfies the DeregisterAgent function on the
// ConsulCatalogAPI interface.
func (c *Consul) deregisterAgent() error {
	c.clientLock.RLock()
	defer c.clientLock.RUnlock()

	if err := c.client.Agent().ServiceDeregister(c.agentID); err != nil {
		return fmt.Errorf("failed to deregister service: %v", err)
	}
	return nil
}
