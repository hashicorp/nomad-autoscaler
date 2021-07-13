package ha

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	uuidParse "github.com/hashicorp/go-uuid"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/file"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/hash"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/uuid"
)

// agentIDFileName is the name of the file used, within the CWD to store
// the agent's unique ID if needed.
const agentIDFileName = "nomad-autoscaler-id"

// DiscoveryRequest is used to monitor the pool of autoscaler agents and report
// on them.
type DiscoveryRequest struct {
	UpdateCh chan<- []hash.RingMember
	ErrCh    chan<- error
}

// PoolDiscovery is the interface which describes the behaviour of a backend
// which supports the autoscaler's HA capabilities. The functionality includes
// discovering and monitoring other agents as well as describing details about
// the agent's self.
type PoolDiscovery interface {

	// AgentID returns the identifier of this agent. The implementation should
	// strive to make this consistent across restarts of this agent. It is
	// required to be unique across agent instances.
	AgentID() string

	// MonitorPool accepts a context and channels for informing the caller of
	// asynchronous updates to the pool.
	MonitorPool(ctx context.Context, req DiscoveryRequest)

	// RegisterAgent registers this agent in the pool. Should makes a
	// best-effort attempt to deregister the agent when the context is closed.
	RegisterAgent(ctx context.Context) error

	// Reload instructs MonitorPool to reload the pool, as a result of updated
	// configuration.
	Reload()

	// WaitForExit provides a blocking function that can be called when
	// ensuring the PoolDiscovery implementation has shut down properly. It is
	// useful where shutdown tasks must/should be completed.
	WaitForExit()
}

// GenerateAgentID generates a unique Nomad Autoscaler agent ID to be used
// wholly, or partially to generate a unique ID. If the autoscaler is running
// on Nomad, we use the allocation ID, otherwise we generate an ID and persist
// this to disk so we can survive restarts.
func GenerateAgentID() (string, error) {

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
