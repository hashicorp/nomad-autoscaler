package consul

import (
	"crypto/sha1"
	"encoding/base32"
	"hash"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/ha"
)

const (
	// autoscalerConsulServiceTag is the Consul service tag used when
	// registering this instance of the autoscaler.
	autoscalerConsulServiceTag = "http"
)

var (
	// b32 is a lowercase base32 encoding for use in URL friendly service hashes
	b32 = base32.NewEncoding(strings.ToLower("abcdefghijklmnopqrstuvwxyz234567"))

	// shutdownTimeout is the maximum time that WaitForExit will block for
	// until it is forced to exit.
	shutdownTimeout = 60 * time.Second
)

// Assert that Consul meets the ha.PoolDiscovery interface.
var _ ha.PoolDiscovery = (*Consul)(nil)

type Consul struct {
	logger hclog.Logger

	// When utilising the client, the clientLock should be used to ensure
	// updates to the client are safe.
	client     *api.Client
	clientLock sync.RWMutex

	// agentID is the unique identifier of this agent amongst autoscaler
	// instances.
	agentID string

	// serviceName is the Consul service registration name used and needs to be
	// consistent across all autoscaler instances.
	serviceName string

	serviceID string

	// agentAddr and agentPort are the HTTP parameters used when setting up the
	// agent HTTP server and are used when creating the Consul check
	// registration.
	agentAddr string
	agentPort int

	// poolDiscoveryDoneCh is used to coordinate shutting down correctly.
	poolDiscoveryDoneCh   chan struct{}
	poolDiscoveryReloadCh chan struct{}
}

func NewConsul(log hclog.Logger, client *api.Client, service string, addr string, port int) (*Consul, error) {

	return &Consul{
		client:              client,
		logger:              log,
		serviceName:         service,
		agentAddr:           addr,
		agentPort:           port,
		poolDiscoveryDoneCh: make(chan struct{}),
	}, nil
}

// UpdateConsulClient satisfies the UpdateConsulClient function on the
// ConsulCatalogAPI interface.
func (c *Consul) UpdateConsulClient(consul *api.Client) {
	c.clientLock.Lock()
	defer c.clientLock.Unlock()

	c.client = consul
}

// generateCheckID ensure we generate a unique Consul checkID by hashing the
// inputs and encoding this into a string.
func generateCheckID(reg api.AgentCheckRegistration, id string) string {
	h := sha1.New()
	hashString(h, id)
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
