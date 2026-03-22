package alert

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"
)

// Fire dispatches an alert to the configured webhook URL.
// It formats the message for ntfy with title, priority, and tags headers.
func Fire(ctx context.Context, rule Rule, target, message string) {
	if rule.Webhook == "" {
		log.Printf("alert: rule %q fired for %s but no webhook configured", rule.Name, target)
		return
	}

	now := time.Now().Local().Format("15:04")

	// Format plain text message
	title := "⚠️ " + rule.Name
	if rule.Severity == "critical" {
		title = "🚨 " + rule.Name
	}

	body := title + "\n" +
		"Target: " + target + "\n" +
		"Severity: " + rule.Severity + "\n" +
		"Time: " + now + "\n\n" +
		message

	// Determine ntfy priority (1=min, 3=default, 5=max)
	priority := "3"
	tags := "warning"
	if rule.Severity == "critical" {
		priority = "urgent"
		tags = "rotating_light,fire"
	} else if rule.Severity == "info" {
		priority = "low"
		tags = "information_source"
	}

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, rule.Webhook, strings.NewReader(body))
	if err != nil {
		log.Printf("alert: create webhook request: %v", err)
		return
	}

	// ntfy headers
	req.Header.Set("Title", rule.Name)
	req.Header.Set("Priority", priority)
	req.Header.Set("Tags", tags)

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
