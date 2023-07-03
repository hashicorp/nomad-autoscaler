package ha

//const (
//	renewalFactor = 0.7
//	waitFactor    = 1.1
//)

//type lock interface {
//	Acquire(ctx context.Context, callerID string) (string, error)
//	Release(ctx context.Context) error
//	Renew(ctx context.Context) error
//}
//
//type HAController struct {
//	ID            string
//	renewalPeriod time.Duration
//	waitPeriod    time.Duration
//	randomDelay   time.Duration
//
//	logger log.Logger
//	lock   lock
//}
//
//func NewHAController(l lock, logger log.Logger, lease time.Duration) *HAController {
//	logger = logger.Named("ha_mode")
//
//	rn := rand.New(rand.NewSource(time.Now().Unix())).Intn(100)
//	hac := HAController{
//		lock:          l,
//		logger:        logger,
//		renewalPeriod: time.Duration(float64(lease) * renewalFactor),
//		waitPeriod:    time.Duration(float64(lease) * waitFactor),
//		ID:            uuid.Generate(),
//		randomDelay:   time.Duration(rn) * time.Millisecond,
//	}
//
//	return &hac
//}
//
//func (hc *HAController) Start(ctx context.Context, protectedFunc func(ctx context.Context)) error {
//	hc.logger.Named(hc.ID)
//
//	// To avoid collisions if all the instances start at the same time, wait
//	// a random time before making the first call.
//	hc.wait(ctx)
//
//	waitTimer := time.NewTimer(hc.waitPeriod)
//	defer waitTimer.Stop()
//
//	for {
//		lockID, err := hc.lock.Acquire(ctx, hc.ID)
//		if err != nil {
//			// TODO: What to do with fatal errors?
//			hc.logger.Error("unable to get lock", err)
//		}
//
//		if lockID != "" {
//			hc.logger.Debug("lock acquired, ID", lockID)
//			funcCtx, cancel := context.WithCancel(ctx)
//			defer cancel()
//
//			// Start running the lock protected function
//			go protectedFunc(funcCtx)
//
//			// Maintain lease is a blocking function, will only return in case
//			// the lock is lost or the context is canceled.
//			err := hc.maintainLease(ctx)
//			if err != nil {
//				hc.logger.Debug("lease lost", err)
//				cancel()
//
//				// Give the protected function some time to return before potentially
//				// running it again.
//				hc.wait(ctx)
//			}
//		}
//
//		if !waitTimer.Stop() {
//			<-waitTimer.C
//		}
//		waitTimer.Reset(hc.waitPeriod)
//
//		select {
//		case <-ctx.Done():
//			hc.logger.Debug("context canceled, returning")
//			return nil
//
//		case <-waitTimer.C:
//			waitTimer.Reset(hc.waitPeriod)
//		}
//	}
//}
//
//func (hc *HAController) maintainLease(ctx context.Context) error {
//	renewTimer := time.NewTimer(hc.renewalPeriod)
//	defer renewTimer.Stop()
//
//	for {
//		select {
//		case <-ctx.Done():
//			hc.logger.Debug("context canceled, returning")
//			return nil
//
//		case <-renewTimer.C:
//			err := hc.lock.Renew(ctx)
//			if err != nil {
//				return err
//			}
//			renewTimer.Reset(hc.renewalPeriod)
//		}
//	}
//
//}
//
//func (hc *HAController) wait(ctx context.Context) {
//	t := time.NewTimer(hc.randomDelay)
//	defer t.Stop()
//
//	select {
//	case <-ctx.Done():
//	case <-t.C:
//	}
//}
