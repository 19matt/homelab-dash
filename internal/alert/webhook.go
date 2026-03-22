package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// webhookPayload is the JSON body sent to webhooks.
type webhookPayload struct {
	Rule      string `json:"rule"`
	Target    string `json:"target"`
	Message   string `json:"message"`
	Severity  string `json:"severity"`
	Timestamp string `json:"timestamp"`
}

// Fire dispatches an alert to the configured webhook URL.
// It logs the result and never panics.
func Fire(ctx context.Context, rule Rule, target, message string) {
	if rule.Webhook == "" {
		log.Printf("alert: rule %q fired for %s but no webhook configured", rule.Name, target)
		return
	}

	payload := webhookPayload{
		Rule:      rule.Name,
		Target:    target,
		Message:   message,
		Severity:  rule.Severity,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("alert: marshal webhook payload: %v", err)
		return
	}

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, rule.Webhook, bytes.NewReader(body))
	if err != nil {
		log.Printf("alert: create webhook request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("alert: webhook POST to %s failed: %v", rule.Webhook, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("alert: webhook fired successfully for rule %q target %s (HTTP %d)", rule.Name, target, resp.StatusCode)
	} else {
		log.Printf("alert: webhook returned HTTP %d for rule %q target %s", resp.StatusCode, rule.Name, target)
	}
}

// FireAsync dispatches a webhook in a goroutine (non-blocking).
func FireAsync(rule Rule, target, message string) {
	go Fire(context.Background(), rule, target, message)
}
