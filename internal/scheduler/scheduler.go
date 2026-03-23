package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
	"github.com/homelab/homelab-dash/internal/collector"
	"github.com/homelab/homelab-dash/internal/config"
	"github.com/homelab/homelab-dash/internal/store"
)

const (
	// DefaultStartupDelayPerChecker is the delay between starting each checker at startup.
	// This prevents all checkers from hitting the network simultaneously.
	DefaultStartupDelayPerChecker = 100 * time.Millisecond

	// DefaultStartupDelayPerCollector is the delay between starting each collector at startup.
	DefaultStartupDelayPerCollector = 100 * time.Millisecond

	// DefaultStartupDelayPerScan is the delay between starting each scan at startup.
	DefaultStartupDelayPerScan = 1 * time.Second

	// DefaultStartupDelayPerEvaluator is the delay between starting each evaluator at startup.
	DefaultStartupDelayPerEvaluator = 1 * time.Second
)

type checkerEntry struct {
	checker  checker.Checker
	interval time.Duration
}

type collectorEntry struct {
	collector collector.Collector
	interval  time.Duration
}

type scanEntry struct {
	engine   ScanRunner
	interval time.Duration
}

// EvalRunner is the interface for alert evaluators that can be scheduled.
type EvalRunner interface {
	Evaluate()
}

// ScanRunner is the interface for scan engines that can be scheduled.
type ScanRunner interface {
	Run(ctx context.Context) error
}

// Scheduler orchestrates periodic execution of checkers, collectors, scans, and evaluations.
type Scheduler struct {
	checkers   []checkerEntry
	collectors []collectorEntry
	scans      []scanEntry
	evaluators []evalEntry
	store      *store.Store
	interval   time.Duration
	broadcast  func(target, check, status string, latencyMs int64)
	configMgr  *config.Manager
	configCh   chan config.ConfigChangeEvent
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	// targetCheckerCancels maps target names to their checker cancel funcs for efficient removal
	targetCheckerCancels map[string][]context.CancelFunc
	// targetCollectorCancels maps target names to their collector cancel funcs for efficient removal
	targetCollectorCancels map[string][]context.CancelFunc
	// targetScanCancels maps target names to their scan cancel funcs for efficient removal
	targetScanCancels map[string][]context.CancelFunc
	mu                sync.RWMutex // protects access to the maps above
}

type evalEntry struct {
	evaluator EvalRunner
	interval  time.Duration
}

// New creates a Scheduler with the given store, default interval, and config manager.
func New(s *store.Store, interval time.Duration, configMgr *config.Manager) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		store:                  s,
		interval:               interval,
		configMgr:              configMgr,
		ctx:                    ctx,
		cancel:                 cancel,
		configCh:               make(chan config.ConfigChangeEvent, 10),
		targetCheckerCancels:   make(map[string][]context.CancelFunc),
		targetCollectorCancels: make(map[string][]context.CancelFunc),
		targetScanCancels:      make(map[string][]context.CancelFunc),
	}
}

// AddChecker registers a checker to run at the default interval.
func (sc *Scheduler) AddChecker(c checker.Checker) {
	sc.checkers = append(sc.checkers, checkerEntry{checker: c, interval: sc.interval})
}

// AddCheckerWithInterval registers a checker to run at a custom interval.
func (sc *Scheduler) AddCheckerWithInterval(c checker.Checker, interval time.Duration) {
	sc.checkers = append(sc.checkers, checkerEntry{checker: c, interval: interval})
}

// AddCollector registers a collector to run at the default interval.
func (sc *Scheduler) AddCollector(c collector.Collector) {
	sc.collectors = append(sc.collectors, collectorEntry{collector: c, interval: sc.interval})
}

// AddCollectorWithInterval registers a collector to run at a custom interval.
func (sc *Scheduler) AddCollectorWithInterval(c collector.Collector, interval time.Duration) {
	sc.collectors = append(sc.collectors, collectorEntry{collector: c, interval: interval})
}

// SetBroadcastFunc sets the function called after each checker result.
func (sc *Scheduler) SetBroadcastFunc(fn func(target, check, status string, latencyMs int64)) {
	sc.broadcast = fn
}

// AddScanEngine registers a scan engine to run at the given interval.
func (sc *Scheduler) AddScanEngine(engine ScanRunner, interval time.Duration) {
	sc.scans = append(sc.scans, scanEntry{engine: engine, interval: interval})
}

// AddEvaluator registers an alert evaluator to run at the given interval.
func (sc *Scheduler) AddEvaluator(evaluator EvalRunner, interval time.Duration) {
	sc.evaluators = append(sc.evaluators, evalEntry{evaluator: evaluator, interval: interval})
}

// Run starts goroutines for each registered checker, collector, scan, and evaluator.
// It blocks until ctx is cancelled.
// It also listens for config changes and updates checkers accordingly.
func (sc *Scheduler) Run(ctx context.Context) {
	// Subscribe to config changes
	configCh := sc.configMgr.NotifyRegister()
	defer func() {
		// Unsubscribe from config changes when done
		// Note: We don't actually unsubscribe in this implementation, but in a more complex system we would
	}()

	// Start all existing checkers, collectors, scans, and evaluators
	// Track them by target for efficient removal
	sc.mu.Lock()
	for _, t := range sc.configMgr.Get().Targets {
		checkers := []checkerEntry{}
		collectors := []collectorEntry{}
		scans := []scanEntry{}

		for _, check := range t.Checks {
			switch check {
			case "ping":
				port := 80
				if len(t.Endpoint) > 0 {
					port = t.Endpoint[0].Port
				}
				ce := checkerEntry{checker: checker.NewPingChecker(t.Name, t.Host, port), interval: sc.interval}
				checkers = append(checkers, ce)
			case "http":
				for _, ep := range t.Endpoint {
					ce := checkerEntry{checker: checker.NewHTTPChecker(t.Name, t.Host, ep.Port, ep.Protocol), interval: sc.interval}
					checkers = append(checkers, ce)
				}
			}
		}

		// Add collectors for this target (if any)
		// Note: Collectors are typically integration-specific and not per-target in the same way
		// For now, we'll leave this empty as the current implementation doesn't have per-target collectors

		// Add scans for this target (if any)
		// Note: Scans are typically not per-target either in this implementation

		// Start the checkers and track their cancel functions
		checkerCancels := make([]context.CancelFunc, 0, len(checkers))
		for i, e := range checkers {
			childCtx, childCancel := context.WithCancel(ctx)
			checkerCancels = append(checkerCancels, childCancel)
			// Stagger checker startup to prevent simultaneous network requests
			startupDelay := time.Duration(i) * DefaultStartupDelayPerChecker
			go sc.runChecker(childCtx, e, startupDelay)
		}

		// Start the collectors and track their cancel functions
		collectorCancels := make([]context.CancelFunc, 0, len(collectors))
		for i, e := range collectors {
			childCtx, childCancel := context.WithCancel(ctx)
			collectorCancels = append(collectorCancels, childCancel)
			// Stagger collector startup to prevent resource contention
			startupDelay := time.Duration(i) * DefaultStartupDelayPerCollector
			go sc.runCollector(childCtx, e, startupDelay)
		}

		// Start the scans and track their cancel functions
		scanCancels := make([]context.CancelFunc, 0, len(scans))
		for i, e := range scans {
			childCtx, childCancel := context.WithCancel(ctx)
			scanCancels = append(scanCancels, childCancel)
			// Stagger scan startup with longer delay (scans are typically more resource-intensive)
			startupDelay := time.Duration(i) * DefaultStartupDelayPerScan
			go sc.runScan(childCtx, e, startupDelay)
		}

		// Store the cancel functions for later use
		sc.targetCheckerCancels[t.Name] = checkerCancels
		sc.targetCollectorCancels[t.Name] = collectorCancels
		sc.targetScanCancels[t.Name] = scanCancels
	}
	sc.mu.Unlock()

	// Start evaluators (these are not target-specific)
	for i, e := range sc.evaluators {
		// Stagger evaluator startup to prevent simultaneous alert evaluation
		startupDelay := time.Duration(i) * DefaultStartupDelayPerEvaluator
		go sc.runEvaluator(ctx, e, startupDelay)
	}

	// Handle config changes and scheduler control in a separate goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case change := <-configCh:
				sc.handleConfigChange(change)
			}
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
}

// handleConfigChange processes config change events and updates the scheduler accordingly.
func (sc *Scheduler) handleConfigChange(event config.ConfigChangeEvent) {
	switch event.Type {
	case config.ChangeTypeAdded:
		// Add checker for new target
		sc.registerTarget(event.Target)
	case config.ChangeTypeRemoved:
		// Remove checker for removed target
		sc.unregisterTarget(event.Target.Name)
	case config.ChangeTypeUpdated:
		// Update checker for modified target
		sc.unregisterTarget(event.OldTarget.Name)
		sc.registerTarget(event.Target)
	}
}

// registerTarget registers checkers, collectors, and scans for a target based on its configuration.
func (sc *Scheduler) registerTarget(target config.TargetConfig) {
	// Register checkers
	checkerCancels := make([]context.CancelFunc, 0)
	for _, check := range target.Checks {
		switch check {
		case "ping":
			port := 80
			if len(target.Endpoint) > 0 {
				port = target.Endpoint[0].Port
			}
			c := checker.NewPingChecker(target.Name, target.Host, port)
			childCtx, childCancel := context.WithCancel(sc.ctx)
			checkerCancels = append(checkerCancels, childCancel)
			// Use existing checker count to calculate startup delay
			existingCheckers := len(sc.targetCheckerCancels[target.Name])
			startupDelay := time.Duration(existingCheckers) * DefaultStartupDelayPerChecker
			go sc.runChecker(childCtx, checkerEntry{checker: c, interval: sc.interval}, startupDelay)
		case "http":
			for _, ep := range target.Endpoint {
				c := checker.NewHTTPChecker(target.Name, target.Host, ep.Port, ep.Protocol)
				childCtx, childCancel := context.WithCancel(sc.ctx)
				checkerCancels = append(checkerCancels, childCancel)
				// Use existing checker count to calculate startup delay
				existingCheckers := len(sc.targetCheckerCancels[target.Name])
				startupDelay := time.Duration(existingCheckers) * DefaultStartupDelayPerChecker
				go sc.runChecker(childCtx, checkerEntry{checker: c, interval: sc.interval}, startupDelay)
			}
		}
	}

	// Note: In a more complete implementation, we would also register collectors and scans per target
	// For now, we'll leave those empty as the current implementation doesn't have per-target collectors/scans

	// Store the cancel functions for later use
	sc.mu.Lock()
	sc.targetCheckerCancels[target.Name] = checkerCancels
	sc.targetCollectorCancels[target.Name] = []context.CancelFunc{} // Empty for now
	sc.targetScanCancels[target.Name] = []context.CancelFunc{}      // Empty for now
	sc.mu.Unlock()
}

// unregisterTarget removes checkers, collectors, and scans for a target by name.
func (sc *Scheduler) unregisterTarget(targetName string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Cancel checkers for this target
	if cancels, ok := sc.targetCheckerCancels[targetName]; ok {
		for _, cancelFunc := range cancels {
			cancelFunc()
		}
		delete(sc.targetCheckerCancels, targetName)
	}

	// Cancel collectors for this target
	if cancels, ok := sc.targetCollectorCancels[targetName]; ok {
		for _, cancelFunc := range cancels {
			cancelFunc()
		}
		delete(sc.targetCollectorCancels, targetName)
	}

	// Cancel scans for this target
	if cancels, ok := sc.targetScanCancels[targetName]; ok {
		for _, cancelFunc := range cancels {
			cancelFunc()
		}
		delete(sc.targetScanCancels, targetName)
	}

	log.Printf("Target %s removed from config. All associated checkers, collectors, and scans have been stopped.", targetName)
}

func (sc *Scheduler) runChecker(ctx context.Context, e checkerEntry, delay time.Duration) {
	if delay > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	sc.executeChecker(ctx, e.checker)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sc.executeChecker(ctx, e.checker)
		}
	}
}

func (sc *Scheduler) executeChecker(ctx context.Context, c checker.Checker) {
	result, err := c.Check(ctx)
	if err != nil {
		log.Printf("checker %s: error: %v", c.Name(), err)
		return
	}

	if err := sc.store.SaveCheckResult(ctx, result); err != nil {
		log.Printf("checker %s: save error: %v", c.Name(), err)
		return
	}

	log.Printf("checker %s: %s/%s = %s (%s)", c.Name(), result.Target, result.Check, result.Status, result.Latency)

	if sc.broadcast != nil {
		sc.broadcast(result.Target, result.Check, result.Status.String(), result.Latency.Milliseconds())
	}
}

func (sc *Scheduler) runCollector(ctx context.Context, e collectorEntry, delay time.Duration) {
	if delay > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	sc.executeCollector(ctx, e.collector)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sc.executeCollector(ctx, e.collector)
		}
	}
}

func (sc *Scheduler) executeCollector(ctx context.Context, c collector.Collector) {
	points, err := c.Collect(ctx)
	if err != nil {
		log.Printf("collector %s: error: %v", c.Name(), err)
		return
	}

	if len(points) > 0 {
		if err := sc.store.SaveDataPoints(ctx, points); err != nil {
			log.Printf("collector %s: save error: %v", c.Name(), err)
			return
		}
	}

	log.Printf("collector %s: collected %d points", c.Name(), len(points))
}

func (sc *Scheduler) runScan(ctx context.Context, e scanEntry, delay time.Duration) {
	if delay > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	sc.executeScan(ctx, e.engine)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sc.executeScan(ctx, e.engine)
		}
	}
}

func (sc *Scheduler) executeScan(ctx context.Context, engine ScanRunner) {
	if err := engine.Run(ctx); err != nil {
		log.Printf("scan engine: error: %v", err)
	}
}

func (sc *Scheduler) runEvaluator(ctx context.Context, e evalEntry, delay time.Duration) {
	if delay > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	e.evaluator.Evaluate()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.evaluator.Evaluate()
		}
	}
}
