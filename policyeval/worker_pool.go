package policyeval

import (
	"context"
	"fmt"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	"github.com/hashicorp/nomad-autoscaler/policy"
)

// defaultHeartbeatInterval is the interval in which workers are expected to
// send heartbeats.
const defaultHeartbeatInterval = 5 * time.Second

// heartbeatTimeout defines how to long to wait for a heartbeat from workers
// before considering it unresponsive.
const heartbeatTimeout = 2 * defaultHeartbeatInterval

// Worker is the interface that defines the set of methods a worker must have.
// TODO(luiz): there's probably a better name for this.
type Worker interface {
	ID() string
	Run(context.Context, time.Duration) <-chan interface{}
}

// WorkerPool manages a set of workers that consume evaluation requests for a
// specific queue.
type WorkerPool struct {
	ctx           context.Context
	logger        hclog.Logger
	pluginManager *manager.PluginManager
	policyManager *policy.Manager
	broker        *Broker
	queue         string

	workersLock sync.RWMutex
	workers     map[string]*workerContext
}

// WorkerPoolConfig holds the basic values required to create a new WorkerPool.
type WorkerPoolConfig struct {
	Logger        hclog.Logger
	PluginManager *manager.PluginManager
	PolicyManager *policy.Manager
	Broker        *Broker
	Queue         string
	Workers       int
}

// workerContext wraps a Worker along with extra controls.
type workerContext struct {
	ctx         context.Context
	cancel      context.CancelFunc
	heartbeatCh <-chan interface{}
	worker      Worker
}

// NewWorkerPool creates a new WorkerPool.
func NewWorkerPool(ctx context.Context, c *WorkerPoolConfig) (*WorkerPool, error) {
	poolCtx, cancel := context.WithCancel(ctx)
	logger := c.Logger.Named("worker_pool").With("queue", c.Queue)

	pool := &WorkerPool{
		ctx:           poolCtx,
		logger:        logger,
		pluginManager: c.PluginManager,
		policyManager: c.PolicyManager,
		broker:        c.Broker,
		queue:         c.Queue,
		workers:       make(map[string]*workerContext, c.Workers),
	}

	err := pool.initWorkers(c.Workers)
	if err != nil {
		cancel()
		return nil, err
	}

	go func() {
		pool.monitorWorkers()
		cancel()
	}()

	return pool, nil
}

// Put adds a new worker to the WorkerPool.
func (wp *WorkerPool) Put() error {
	wp.workersLock.Lock()
	defer wp.workersLock.Unlock()

	wp.logger.Debug("adding new worker to the pool")

	w, err := wp.createWorker(wp.queue)
	if err != nil {
		return fmt.Errorf("failed to put new worker in pool: %v", err)
	}

	wCtx, wCancel := context.WithCancel(wp.ctx)
	wHearbeatCh := w.Run(wCtx, defaultHeartbeatInterval)
	id := w.ID()

	wp.workers[id] = &workerContext{
		ctx:         wCtx,
		cancel:      wCancel,
		heartbeatCh: wHearbeatCh,
		worker:      w,
	}

	return nil
}

func (wp *WorkerPool) initWorkers(count int) error {
	for i := 0; i < count; i++ {
		if err := wp.Put(); err != nil {
			return err
		}
	}
	return nil
}

func (wp *WorkerPool) createWorker(queue string) (Worker, error) {
	switch queue {
	case "horizontal", "cluster":
		return NewBaseWorker(wp.logger, wp.pluginManager, wp.policyManager, wp.broker, queue), nil
	default:
		return nil, fmt.Errorf("invalid queue %q", queue)
	}
}

func (wp *WorkerPool) monitorWorkers() {
	for {
		for id, w := range wp.workers {
			select {
			case <-wp.ctx.Done():
				return
			case <-time.After(heartbeatTimeout):
				wp.removeWorker(id)
			case <-w.heartbeatCh:
				wp.logger.Trace("received worker heartbeat", "worker_id", id)
			}
		}
	}
}

func (wp *WorkerPool) removeWorker(id string) {
	wp.workersLock.Lock()
	defer wp.workersLock.Unlock()

	wp.logger.Debug("removing worker", "worker_id", id)

	w := wp.workers[id]
	w.cancel()
	delete(wp.workers, id)
}
