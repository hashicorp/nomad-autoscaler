package ha

import (
	"context"
	"crypto/sha1"
	"encoding/base32"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	uuidParse "github.com/hashicorp/go-uuid"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/file"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/uuid"
)

const (
	// autoscalerConsulServiceTag is the Consul service tag used when
	// registering this instance of the autoscaler.
	autoscalerConsulServiceTag = "http"

	// agentIDFileName is the name of the file used, within the CWD to store
	// the agent's unique ID if needed.
	agentIDFileName = "nomad-autoscaler-id"
)

var (
	// shutdownTimeout is the maximum time that WaitForExit will block for
	// until it is forced to exit.
	shutdownTimeout = 60 * time.Second

	// b32 is a lowercase base32 encoding for use in URL friendly service hashes
	b32 = base32.NewEncoding(strings.ToLower("abcdefghijklmnopqrstuvwxyz234567"))
)

// consulDiscovery is the Consul implementation of the PoolDiscovery interface
// and discovers autoscaler agents using the Consul service catalog.
type consulDiscovery struct {
	consulCatalog ConsulCatalogAPI
	reloadCh      chan struct{}

	// serviceName is the Consul service registration name used and needs to be
	// consistent across all autoscaler instances.
	serviceName string

	// agentID is the unique identifier of this agent amongst autoscaler
	// instances.
	agentID string

	// agentAddr and agentPort are the HTTP parameters used when setting up the
	// agent HTTP server and are used when creating the Consul check
	// registration.
	agentAddr string
	agentPort int

	// doneCh is used to coordinate shutting down correctly.
	doneCh chan struct{}
}

// NewConsulDiscovery returns a new consulDiscovery implementation of the
// PoolDiscovery interface.
func NewConsulDiscovery(consulCatalog ConsulCatalogAPI, serviceName, addr string, port int) PoolDiscovery {
	return &consulDiscovery{
		consulCatalog: consulCatalog,
		serviceName:   serviceName,
		agentAddr:     addr,
		agentPort:     port,
		doneCh:        make(chan struct{}),
	}
}

// Reload satisfies the Reload function on the PoolDiscovery interface.
func (cd *consulDiscovery) Reload() { cd.reloadCh <- struct{}{} }

// WaitForExit satisfies the WaitForExit function on the PoolDiscovery
// interface.
func (cd *consulDiscovery) WaitForExit() {
	select {
	case <-time.After(shutdownTimeout):
	case <-cd.doneCh:
	}
}

// SetConsulClient satisfies the SetConsulClient function on the PoolDiscovery
// interface.
func (cd *consulDiscovery) SetConsulClient(consul *api.Client) {
	cd.consulCatalog.UpdateConsulClient(consul)
}

// AgentID satisfies the AgentID function on the PoolDiscovery interface.
func (cd *consulDiscovery) AgentID() string { return cd.agentID }

// MonitorPool satisfies the MonitorPool function on the PoolDiscovery
// interface.
func (cd *consulDiscovery) MonitorPool(ctx context.Context, req discoveryRequest) {
	q := &api.QueryOptions{
		AllowStale: true,
		WaitIndex:  0,
		WaitTime:   5 * time.Minute,
	}
	for {
		errCh := make(chan error)
		resCh := make(chan []*api.CatalogService)
		go func() {
			services, qm, err := cd.consulCatalog.QueryService(cd.serviceName, autoscalerConsulServiceTag, q)
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
			pool := make([]agentID, 0, len(services))
			for _, s := range services {
				pool = append(pool, agentID(s.ServiceID))
			}
			req.updateCh <- pool
		}
	}
}

// RegisterAgent satisfies the RegisterAgent function on the PoolDiscovery
// interface.
func (cd *consulDiscovery) RegisterAgent(ctx context.Context) error {

	if err := cd.setupAgentID(); err != nil {
		return fmt.Errorf("failed to setup agent: %v", err)
	}

	// TODO (jrasell) we should add basic retry.
	if err := cd.consulCatalog.RegisterAgent(cd.serviceName, cd.AgentID(), cd.agentAddr, cd.agentPort); err != nil {
		return err
	}

	// Start a goroutine to deregister me on context cancellation. This is best
	// effort only. We should not remove the ID file if it exists. When running
	// on metal, shutdown of the autoscaler could be due to maintenance and we
	// want to come back up with the same ID; like how Nomad does.
	go func() {
		<-ctx.Done()
		_ = cd.consulCatalog.DeregisterAgent(cd.serviceName, cd.AgentID())
		cd.doneCh <- struct{}{}
	}()

	return nil
}

// setupAgentID generates the unique Nomad Autoscaler agent ID. If the
// autoscaler is running on Nomad, we use the allocation ID, otherwise we
// generate an ID and persist this to disk so we can survive restarts.
func (cd *consulDiscovery) setupAgentID() error {

	// Check for an allocation ID environment variable and use this if found.
	if id := os.Getenv("NOMAD_ALLOCATION_ID"); id != "" {
		cd.agentID = id
		return nil
	}

	// If we are not running as a Nomad allocation, identify the current
	// working directory and attempt to use a well known file to persist the
	// agent ID.
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	idFile := filepath.Join(pwd, agentIDFileName)

	if _, err := os.Stat(idFile); err == nil {
		rawID, err := ioutil.ReadFile(idFile)
		if err != nil {
			return err
		}

		// Sanitize the input, as if it were your hands during covid times.
		nodeID := strings.ToLower(strings.TrimSpace(string(rawID)))

		if _, err := uuidParse.ParseUUID(nodeID); err != nil {
			return err
		}
		cd.agentID = nodeID
		return nil
	}

	// If we do not have an agentID at this point, create one and write this to
	// disk.
	if cd.agentID == "" {
		id := uuid.Generate()

		if err := file.EnsurePath(idFile, false); err != nil {
			return err
		}
		if err := ioutil.WriteFile(idFile, []byte(id), 0600); err != nil {
			return err
		}
		cd.agentID = id
	}
	return nil
}

// defaultConsulCatalog is the Consul implementation of the ConsulCatalogAPI
// interface.
type defaultConsulCatalog struct {
	consulClient *api.Client
}

// NewDefaultConsulCatalog returns a new defaultConsulCatalog implementation of
// the ConsulCatalogAPI interface.
func NewDefaultConsulCatalog(consul *api.Client) ConsulCatalogAPI {
	return &defaultConsulCatalog{
		consulClient: consul,
	}
}

// DeregisterAgent satisfies the DeregisterAgent function on the
// ConsulCatalogAPI interface.
func (d *defaultConsulCatalog) DeregisterAgent(service, agentID string) error {

	id := generateServiceID(service, agentID)

	if err := d.consulClient.Agent().ServiceDeregister(id); err != nil {
		return fmt.Errorf("failed to deregister service: %v", err)
	}
	return nil
}

// Datacenters satisfies the Datacenters function on the ConsulCatalogAPI
// interface.
func (d *defaultConsulCatalog) Datacenters() ([]string, error) {
	return d.consulClient.Catalog().Datacenters()
}

// QueryService satisfies the QueryService function on the ConsulCatalogAPI
// interface.
func (d *defaultConsulCatalog) QueryService(service, tag string, q *api.QueryOptions) ([]*api.CatalogService, *api.QueryMeta, error) {
	services, meta, err := d.consulClient.Catalog().Service(service, tag, q)
	return services, meta, err
}

// RegisterAgent satisfies the RegisterAgent function on the ConsulCatalogAPI
// interface.
func (d *defaultConsulCatalog) RegisterAgent(service, agentID, addr string, port int) error {

	// Generate our ID.
	id := generateServiceID(service, agentID)

	// Generate a service registration for this agent. The meta provides an
	// insight to operators as to where it came from, but as yet we don't have
	// the flashy label Nomad has.
	serviceReg := api.AgentServiceRegistration{
		ID:      id,
		Name:    "nomad-autoscaler-agent",
		Tags:    []string{autoscalerConsulServiceTag},
		Address: addr,
		Port:    port,
		Meta:    map[string]string{"external-source": "nomad-autoscaler"},
	}

	// Generate a check registration object, ensuring the ID is unique.
	checkReg := api.AgentCheckRegistration{
		Name:      "nomad-autoscaler-agent",
		ServiceID: serviceReg.ID,
	}
	checkReg.ID = generateCheckID(id, checkReg)
	checkReg.TCP = net.JoinHostPort(addr, strconv.Itoa(port))
	checkReg.Interval = "10s"

	// Register and format the errors to provide clearer feedback.
	if err := d.consulClient.Agent().ServiceRegister(&serviceReg); err != nil {
		return fmt.Errorf("failed to register service: %v", err)
	}
	if err := d.consulClient.Agent().CheckRegister(&checkReg); err != nil {
		return fmt.Errorf("failed to register check: %v", err)
	}
	return nil
}

// UpdateConsulClient satisfies the UpdateConsulClient function on the
// ConsulCatalogAPI interface.
func (d *defaultConsulCatalog) UpdateConsulClient(consul *api.Client) { d.consulClient = consul }

// generateServiceID ensures we generate consistent Consul service IDs when
// performing API operations.
func generateServiceID(service, agentID string) string { return "_" + service + "_" + agentID }

// generateCheckID ensure we generate a unique Consul checkID by hashing the
// inputs and encoding this into a string.
func generateCheckID(serviceID string, reg api.AgentCheckRegistration) string {
	h := sha1.New()
	hashString(h, serviceID)
	hashString(h, reg.Name)
	hashString(h, reg.TCP)
	hashString(h, reg.Interval)

	// Base32 is used for encoding the hash as sha1 hashes can always be
	// encoded without padding, only 4 bytes larger than base64, and saves
	// 8 bytes vs hex. Since these hashes are used in Consul URLs it's nice
	// to have a reasonably compact URL-safe representation.
	return b32.EncodeToString(h.Sum(nil))
}

func hashString(h hash.Hash, s string) { _, _ = io.WriteString(h, s) }
