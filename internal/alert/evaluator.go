package alert

import (
	"context"
	"log"
	"time"

	"github.com/homelab/homelab-dash/internal/scanner"
	"github.com/homelab/homelab-dash/internal/store"
)

// Evaluator wraps an alert Engine and implements the scheduler.EvalRunner interface.
type Evaluator struct {
	engine *Engine
	store  *store.Store
}

// NewEvaluator creates a new alert evaluator.
func NewEvaluator(engine *Engine, s *store.Store) *Evaluator {
	return &Evaluator{
		engine: engine,
		store:  s,
	}
}

// Evaluate fetches latest data from the store and runs alert evaluation.
func (e *Evaluator) Evaluate() {
	log.Println("alert eval: starting evaluation cycle")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch latest check results
	results, err := e.store.GetLatestCheckResults(ctx)
	if err != nil {
		log.Printf("alert eval: get check results: %v", err)
		return
	}
	log.Printf("alert eval: fetched %d check results", len(results))

	// Fetch current security findings
	findings, err := e.store.GetFindings(ctx)
	if err != nil {
		log.Printf("alert eval: get findings: %v", err)
		return
	}
	log.Printf("alert eval: fetched %d findings", len(findings))

	// Convert store findings to scanner findings for the engine
	var scannerFindings []scanner.Finding
	for _, f := range findings {
		scannerFindings = append(scannerFindings, scanner.Finding{
			Target:      f.Target,
			Title:       f.Title,
			Severity:    scanner.Severity(f.Severity),
			Description: f.Description,
		})
	}

	// Evaluate (points are fetched per-rule in the engine via store)
	fired := e.engine.Evaluate(results, nil, scannerFindings)

	if len(fired) > 0 {
		log.Printf("alert eval: %d alerts fired", len(fired))
	} else {
		log.Println("alert eval: no alerts fired")
	}

	for _, alert := range fired {
		log.Printf("alert: FIRED rule=%q target=%s message=%q", alert.Rule.Name, alert.Target, alert.Message)

		// Save to store
		event := store.AlertEvent{
			Timestamp: alert.FiredAt,
			RuleName:  alert.Rule.Name,
			Target:    alert.Target,
			Message:   alert.Message,
			Severity:  alert.Rule.Severity,
		}
		if err := e.store.SaveAlertEvent(ctx, event); err != nil {
			log.Printf("alert: save event error: %v", err)
		}

		// Fire webhook asynchronously
		if alert.Rule.Webhook != "" {
			FireAsync(alert.Rule, alert.Target, alert.Message)
		}
	}
}
