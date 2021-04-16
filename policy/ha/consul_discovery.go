package ha

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/consul/api"
)

var (
	AutoscalerConsulServiceTag = "http"
)

type consulDiscovery struct {
	consulCatalog ConsulCatalogAPI
	serviceName   string
	reloadCh      chan struct{}
}

func (cd *consulDiscovery) Reload() {
	cd.reloadCh <- struct{}{}
}

func (cd *consulDiscovery) SetConsulClient(consul *api.Client) {
	cd.consulCatalog.UpdateConsulClient(consul)
}

func (cd *consulDiscovery) AgentID() string {
	// TODO: this assumes that we're running as a nomad job
	// long term, add a fall-back, like a randomly-generated UUID which we can
	// persist to disk for restarts
	return os.Getenv("NOMAD_ALLOCATION_ID")
}

func (cd *consulDiscovery) MonitorPool(ctx context.Context, req DiscoveryRequest) {
	q := &api.QueryOptions{
		AllowStale: true,
		WaitIndex:  0,
		WaitTime:   5 * time.Minute,
	}
	for {
		errCh := make(chan error)
		resCh := make(chan []*api.CatalogService)
		go func() {
			services, qm, err := cd.consulCatalog.QueryService(cd.serviceName, AutoscalerConsulServiceTag, q)
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
		case <-cd.reloadCh:
			continue
		case err := <-errCh:
			req.errCh <- fmt.Errorf("error querying Consul catalog: %v", err)
			time.Sleep(5 * time.Second) // maybe a backoff here?
		case services := <-resCh:
			pool := make([]AgentID, 0, len(services))
			for _, s := range services {
				pool = append(pool, AgentID(s.ServiceID))
			}
			req.updateCh <- pool
		}
	}
}

func (cd *consulDiscovery) RegisterAgent(ctx context.Context) error {
	if err := cd.consulCatalog.RegisterAgent(cd.serviceName, cd.AgentID()); err != nil {
		// TODO: maybe consider retrying?
		return err
	}

	// start goroutine to deregister me on context cancellation
	go func() {
		<-ctx.Done()
		cd.consulCatalog.DeregisterAgent(cd.serviceName, cd.AgentID())
	}()

	return nil
}

func NewConsulDiscovery(consulCatalog ConsulCatalogAPI, serviceName string) PoolDiscovery {
	return &consulDiscovery{
		consulCatalog: consulCatalog,
		serviceName:   serviceName,
	}
}

type DefaultConsulCatalog struct {
	consulClient *api.Client
}

func (d *DefaultConsulCatalog) DeregisterAgent(service, agentID string) error {
	// FINISH: deregister
	// steal this code from nomad: https://github.com/hashicorp/nomad/blob/main/command/agent/agent.go#L706
	panic("implement me")
}

func NewDefaultConsulCatalog(consul *api.Client) ConsulCatalogAPI {
	return &DefaultConsulCatalog{
		consulClient: consul,
	}
}

func (d *DefaultConsulCatalog) Datacenters() ([]string, error) {
	dcs, err := d.consulClient.Catalog().Datacenters()
	return dcs, err
}

func (d *DefaultConsulCatalog) QueryService(service, tag string, q *api.QueryOptions) ([]*api.CatalogService, *api.QueryMeta, error) {
	services, meta, err := d.consulClient.Catalog().Service(service, tag, q)
	return services, meta, err
}

func (d *DefaultConsulCatalog) RegisterAgent(service, agentID string) error {
	// FINISH: register this agent service against the consul cluster
	// steal this code from Nomad: https://github.com/hashicorp/nomad/blob/main/command/agent/agent.go#L706
	panic("implement me")
}

func (d *DefaultConsulCatalog) UpdateConsulClient(consul *api.Client) {
	d.consulClient = consul
}
