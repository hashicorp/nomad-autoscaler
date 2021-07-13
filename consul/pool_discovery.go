package consul

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
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
		WaitIndex: 0,
		WaitTime:  5 * time.Minute,
	}
	for {

		// Generate a child context and configure the Consul query options to
		// utilise this. It allows us to cancel the blocking query as a result
		// of reloads or shutdowns.
		consulCtx, consulCancel := context.WithCancel(ctx)
		q = q.WithContext(consulCtx)

		errCh := make(chan error)
		resCh := make(chan []*api.CatalogService)

		go func() {

			c.clientLock.RLock()
			defer c.clientLock.RUnlock()

			// This routine writes to the channels, so therefore is responsible
			// for closing them on exit.
			defer close(errCh)
			defer close(resCh)

			services, qm, err := c.client.Catalog().Service(c.serviceName, autoscalerConsulServiceTag, q)

			// If the context has been cancelled, exit.
			if consulCtx.Err() != nil {
				return
			}

			if err != nil {
				errCh <- err
			} else {
				q.WaitIndex = qm.LastIndex
				resCh <- services
			}
		}()
		select {
		case <-ctx.Done():
			consulCancel()
			return
		case <-c.poolDiscoveryReloadCh:
			consulCancel()
			continue
		case err := <-errCh:
			req.ErrCh <- fmt.Errorf("error querying Consul catalog: %v", err)
			time.Sleep(5 * time.Second)
		case services := <-resCh:
			pool := make([]hash.RingMember, len(services))
			for i, service := range services {
				pool[i] = hash.NewRingMember(agentIDFromServiceID(service.ServiceID))
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
	c.serviceID = c.generateServiceID()

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
		ID:      c.serviceID,
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
		AgentServiceCheck: api.AgentServiceCheck{
			Interval: "10s",
			TCP:      net.JoinHostPort(c.agentAddr, strconv.Itoa(c.agentPort)),
		},
	}
	checkReg.ID = generateCheckID(checkReg, c.agentID)

	c.clientLock.RLock()
	defer c.clientLock.RUnlock()

	// Retry and backoff when attempting to register the service within Consul.
	serviceFunc := func() error { return c.client.Agent().ServiceRegister(&serviceReg) }

	if err := backoff.Retry(serviceFunc, generateRetryBackoff()); err != nil {
		return fmt.Errorf("failed to register service: %v", err)
	}

	// Retry and backoff when attempting to register the service check within
	// Consul.
	checkFunc := func() error { return c.client.Agent().CheckRegister(&checkReg) }

	if err := backoff.Retry(checkFunc, generateRetryBackoff()); err != nil {
		return fmt.Errorf("failed to register service check: %v", err)
	}

	c.logger.Info("successfully registered agent", "service_id", c.serviceID)
	return nil
}

// DeregisterAgent satisfies the DeregisterAgent function on the
// ConsulCatalogAPI interface.
func (c *Consul) deregisterAgent() error {
	c.clientLock.RLock()
	defer c.clientLock.RUnlock()

	if err := c.client.Agent().ServiceDeregister(c.serviceID); err != nil {
		return fmt.Errorf("failed to deregister service: %v", err)
	}
	c.logger.Info("successfully deregistered agent", "service_id", c.serviceID)
	return nil
}

// generateServiceID creates the service identifier to use when registering an
// agent within Consul. It follows the same pattern as Nomad and is a composite
// including the agents unique ID.
func (c *Consul) generateServiceID() string {
	return fmt.Sprintf("_%s_%s", c.serviceName, c.agentID)
}

// agentIDFromServiceID is used to obtain the agentID from the Consul service
// ID. Please see Consul.generateServiceID for how the serviceID is generated.
func agentIDFromServiceID(serviceID string) string {
	parts := strings.Split(serviceID, "_")
	return parts[len(parts)-1]
}

// generateRetryBackoff creates the backoff.ExponentialBackOff object used when
// attempting to register the agent within Consul. It currently adjusts the max
// elapsed time allowed from 15 mins to 10 seconds.
func generateRetryBackoff() *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 10 * time.Second
	return b
}
