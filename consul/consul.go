package consul

import (
	"crypto/sha1"
	"encoding/base32"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
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
	// b32 is a lowercase base32 encoding for use in URL friendly service hashes
	b32 = base32.NewEncoding(strings.ToLower("abcdefghijklmnopqrstuvwxyz234567"))

	// shutdownTimeout is the maximum time that WaitForExit will block for
	// until it is forced to exit.
	shutdownTimeout = 60 * time.Second
)

type Consul struct {
	logger hclog.Logger

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
	agentID, err := setupAgentID()
	if err != nil {
		return nil, err
	}

	return &Consul{
		client:              client,
		agentID:             agentID,
		serviceName:         service,
		serviceID:           fmt.Sprintf("_%s_%s", service, agentID),
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

// setupAgentID generates the unique Nomad Autoscaler agent ID. If the
// autoscaler is running on Nomad, we use the allocation ID, otherwise we
// generate an ID and persist this to disk so we can survive restarts.
func setupAgentID() (string, error) {

	// Check for an allocation ID environment variable and use this if found.
	if id := os.Getenv("NOMAD_ALLOCATION_ID"); id != "" {
		return id, nil
	}

	// If we are not running as a Nomad allocation, identify the current
	// working directory and attempt to use a well known file to persist the
	// agent ID.
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	idFile := filepath.Join(pwd, agentIDFileName)

	if _, err := os.Stat(idFile); err == nil {
		rawID, err := ioutil.ReadFile(idFile)
		if err != nil {
			return "", err
		}

		// Sanitize the input, as if it were your hands during covid times.
		nodeID := strings.ToLower(strings.TrimSpace(string(rawID)))

		if _, err := uuidParse.ParseUUID(nodeID); err != nil {
			return "", err
		}
		return nodeID, nil
	}

	// If we do not have an agentID at this point, create one and write this to
	// disk.
	id := uuid.Generate()

	if err := file.EnsurePath(idFile, false); err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(idFile, []byte(id), 0600); err != nil {
		return "", err
	}
	return id, nil
}

// generateCheckID ensure we generate a unique Consul checkID by hashing the
// inputs and encoding this into a string.
func (c *Consul) generateCheckID(reg api.AgentCheckRegistration) string {
	h := sha1.New()
	hashString(h, c.serviceID)
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
