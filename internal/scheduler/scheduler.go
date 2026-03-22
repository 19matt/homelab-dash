package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
	"github.com/homelab/homelab-dash/internal/collector"
	"github.com/homelab/homelab-dash/internal/store"
)

type checkerEntry struct {
	checker  checker.Checker
	interval time.Duration
}

type collectorEntry struct {
	collector collector.Collector
	interval  time.Duration
}

// Scheduler orchestrates periodic execution of checkers and collectors.
type Scheduler struct {
	checkers   []checkerEntry
	collectors []collectorEntry
	store      *store.Store
	interval   time.Duration
	broadcast  func(target, check, status string, latencyMs int64)
}

// New creates a Scheduler with the given store and default interval.
func New(s *store.Store, interval time.Duration) *Scheduler {
	return &Scheduler{
		store:    s,
		interval: interval,
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

// Run starts goroutines for each registered checker and collector.
// It blocks until ctx is cancelled.
func (sc *Scheduler) Run(ctx context.Context) {
	for i, e := range sc.checkers {
		go sc.runChecker(ctx, e, time.Duration(i)*100*time.Millisecond)
	}
	for i, e := range sc.collectors {
		go sc.runCollector(ctx, e, time.Duration(i)*100*time.Millisecond)
	}
	<-ctx.Done()
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
