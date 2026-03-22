package scanner

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/homelab/homelab-dash/internal/config"
	"github.com/homelab/homelab-dash/internal/store"
)

// ScanEngine orchestrates security scans against configured targets.
type ScanEngine struct {
	scanners []Scanner
	targets  []config.TargetConfig
	store    *store.Store
}

// NewEngine creates a new scan engine.
func NewEngine(s *store.Store, targets []config.TargetConfig) *ScanEngine {
	return &ScanEngine{
		targets: targets,
		store:   s,
	}
}

// AddScanner registers a scanner to run.
func (e *ScanEngine) AddScanner(sc Scanner) {
	e.scanners = append(e.scanners, sc)
}

// RunAll runs all scanners against all targets concurrently and saves findings.
// Previous findings are cleared and tracked in history.
func (e *ScanEngine) RunAll(ctx context.Context) ([]Finding, error) {
	scanID := time.Now().Format("20060102-150405")

	var mu sync.Mutex
	var allFindings []Finding

	var wg sync.WaitGroup
	for _, target := range e.targets {
		wg.Add(1)
		go func(t config.TargetConfig) {
			defer wg.Done()

			for _, sc := range e.scanners {
				findings, err := sc.Scan(ctx, t)
				if err != nil {
					log.Printf("scanner %s: error scanning %s: %v", sc.Name(), t.Name, err)
					continue
				}

				mu.Lock()
				allFindings = append(allFindings, findings...)
				mu.Unlock()
			}
		}(target)
	}
	wg.Wait()

	// Convert to store findings with scan ID
	storeFindings := make([]store.Finding, len(allFindings))
	for i, f := range allFindings {
		storeFindings[i] = store.Finding{
			Timestamp:   f.Timestamp,
			Target:      f.Target,
			Scanner:     f.Scanner,
			Title:       f.Title,
			Description: f.Description,
			Severity:    int(f.Severity),
			Remediation: f.Remediation,
			ScanID:      scanID,
		}
	}

	// Save findings (clears old ones, tracks history)
	if err := e.store.SaveFindings(ctx, storeFindings); err != nil {
		return allFindings, fmt.Errorf("scan engine: save findings: %w", err)
	}

	log.Printf("scan engine: completed scan %s, found %d findings across %d targets", scanID, len(allFindings), len(e.targets))
	return allFindings, nil
}

// Run executes all scans and discards the findings (for scheduler interface).
func (e *ScanEngine) Run(ctx context.Context) error {
	_, err := e.RunAll(ctx)
	return err
}
