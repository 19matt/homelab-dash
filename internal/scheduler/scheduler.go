package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
	"github.com/homelab/homelab-dash/internal/collector"
	"github.com/homelab/homelab-dash/internal/store"
)

// Scheduler orchestrates periodic execution of checkers and collectors.
type Scheduler struct {
	checkers   []checker.Checker
	collectors []collector.Collector
	store      *store.Store
	interval   time.Duration
}

// New creates a Scheduler with the given store and interval.
func New(s *store.Store, interval time.Duration) *Scheduler {
	return &Scheduler{
		store:    s,
		interval: interval,
	}
}

// AddChecker registers a checker to run on the schedule.
func (sc *Scheduler) AddChecker(c checker.Checker) {
	sc.checkers = append(sc.checkers, c)
}

// AddCollector registers a collector to run on the schedule.
func (sc *Scheduler) AddCollector(c collector.Collector) {
	sc.collectors = append(sc.collectors, c)
}

// Run starts goroutines for each registered checker and collector.
// It blocks until ctx is cancelled.
func (sc *Scheduler) Run(ctx context.Context) {
	for i, c := range sc.checkers {
		go sc.runChecker(ctx, c, time.Duration(i)*100*time.Millisecond)
	}
	for i, c := range sc.collectors {
		go sc.runCollector(ctx, c, time.Duration(i)*100*time.Millisecond)
	}
	<-ctx.Done()
}

func (sc *Scheduler) runChecker(ctx context.Context, c checker.Checker, delay time.Duration) {
	// Stagger initial runs to avoid concurrent DB writes
	if delay > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}

	ticker := time.NewTicker(sc.interval)
	defer ticker.Stop()

	sc.executeChecker(ctx, c)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sc.executeChecker(ctx, c)
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
}

func (sc *Scheduler) runCollector(ctx context.Context, c collector.Collector, delay time.Duration) {
	if delay > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}

	ticker := time.NewTicker(sc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := c.Collect(ctx)
			if err != nil {
				log.Printf("collector %s: error: %v", c.Name(), err)
			}
		}
	}
}
