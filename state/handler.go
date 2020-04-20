package state

import (
	"context"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/state/policy"
	"github.com/hashicorp/nomad-autoscaler/state/policy/source"
	nomadSource "github.com/hashicorp/nomad-autoscaler/state/policy/source/nomad"
	"github.com/hashicorp/nomad-autoscaler/state/status"
	"github.com/hashicorp/nomad/api"
)

// HandlerConfig stores configuration values for a Handler.
type HandlerConfig struct {
	Logger             hclog.Logger
	NomadClient        *api.Client
	EvaluationInterval time.Duration
}

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

	// defaultEvaluationInterval is the interval used to evaluate a policy if
	// one is not defined by the policy itself.
	defaultEvaluationInterval time.Duration

	// PolicyState is the interface for interacting with the internal policy
	// state. It is public as the agent needs to be able to list policies.
	PolicyState policy.State

	policyReconcileChan chan []*api.ScalingPolicyListStub

	// policyUpdateChan is the channel where policy.Watcher process should send
	// any updates to scaling policies. The state handler is responsible for
	// listening to this, and processing any items on the channel.
	policyUpdateChan chan *api.ScalingPolicy

	// policySource is the backend implementation which is responsible for
	// retrieving policies from their canonical source.
	policySource source.PolicySource

	// statusState is the interface for interacting with the internal job scale
	// status state.
	statusState status.State

	// statusUpdateChan is the channel where the status.Watcher process should
	// send any updates to the scaling status of a job.
	statusUpdateChan chan *api.JobScaleStatusResponse

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
func NewHandler(ctx context.Context, config *HandlerConfig) *Handler {
	h := Handler{
		ctx:                       ctx,
		log:                       config.Logger.Named("state_handler"),
		nomad:                     config.NomadClient,
		defaultEvaluationInterval: config.EvaluationInterval,
		PolicyState:               policy.NewStateBackend(),
		policyReconcileChan:       make(chan []*api.ScalingPolicyListStub),
		policyUpdateChan:          make(chan *api.ScalingPolicy, 10),
		statusWatcherHandlers:     make(map[string]*status.Watcher),
		statusState:               status.NewStateBackend(),
		statusUpdateChan:          make(chan *api.JobScaleStatusResponse, 10),
	}

	h.policySource = nomadSource.NewNomadPolicySource(h.log, h.nomad)

	return &h
}

// Start starts the initially required state handling processes.
func (h *Handler) Start() {

	// Start the policy and job status update handlers before anything else.
	go h.policyUpdateHandler()
	go h.jobStatusUpdateHandler()

	// The policy reconciliation handler should be started before the policy
	// store is triggered.
	go h.policyReconciliationHandler()

	// The policy source runs as a single process per Autoscaler agent, start
	// it now.
	go h.policySource.Start(h.policyUpdateChan, h.policyReconcileChan)

	// Start the GC loop last, this is the least critical and runs in the
	// background periodically.
	go h.garbageCollectStateLoop()
}
