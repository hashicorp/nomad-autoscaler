package state

import (
	"context"
	"sync"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/state/policy"
	"github.com/hashicorp/nomad-autoscaler/state/status"
	"github.com/hashicorp/nomad/api"
)

// Handler manages the internal state storage for the Autoscaler agent and
// should be used to ensure required information from Nomad (our source of
// truth) is stored locally. This reduces pressure on the Nomad API and allows
// for faster lookups.
type Handler struct {

	// ctx is the context passed by the controlling agent in order to propagate
	// shutdown.
	//
	// TODO(jrasell) we should figure out how to shutdown blocking queries
	//  based on this.
	ctx context.Context

	log   hclog.Logger
	nomad *api.Client

	// PolicyState is the interface for interacting with the internal policy
	// state. It is public as the agent needs to be able to list policies.
	PolicyState policy.State

	// policyUpdateChan is the channel where policy.Watcher process should send
	// any updates to scaling policies. The state handler is responsible for
	// listening to this, and processing any items on the channel.
	policyUpdateChan chan *api.ScalingPolicy

	// policyWatcher is the handle to the blocking query process monitoring the
	// Nomad scaling policy API for changes.
	policyWatcher *policy.Watcher

	// statusState is the interface for interacting with the internal job scale
	// status state.
	statusState status.State

	// statusWatcherHandlerLock is the mutex which should be used when
	// manipulating the statusWatcherHandlers map.
	statusWatcherHandlerLock sync.RWMutex

	// statusWatcherHandlers is a mapping on our job scaling status blocking
	// query handlers. Each job which is running and configured with at least
	// one scaling policy should have an associated watcher. The map is keyed
	// by the JobID.
	statusWatcherHandlers map[string]*status.Watcher
}

// NewHandler is used to build a new state Handler object for use in managing
// the Autoscaler internal state.
func NewHandler(ctx context.Context, log hclog.Logger, nomad *api.Client) *Handler {
	h := Handler{
		ctx:                   ctx,
		log:                   log.Named("state_handler"),
		nomad:                 nomad,
		PolicyState:           policy.NewStateBackend(),
		policyUpdateChan:      make(chan *api.ScalingPolicy, 10),
		statusWatcherHandlers: make(map[string]*status.Watcher),
		statusState:           status.NewStateBackend(),
	}

	h.policyWatcher = policy.NewWatcher(log, nomad, h.policyUpdateChan)

	return &h
}

// Start starts the initially required state handling processes.
func (h *Handler) Start() {

	// Start the policy update handler before anything else.
	go h.policyUpdateHandler()

	// The policy watcher runs as a single process per Autoscaler agent, start
	// it now.
	go h.policyWatcher.Start()
}
