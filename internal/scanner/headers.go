package scanner

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/homelab/homelab-dash/internal/config"
)

// HeaderScanner checks HTTP response headers for security best practices.
type HeaderScanner struct{}

// Name returns the scanner name.
func (h *HeaderScanner) Name() string {
	return "headers"
}

type headerCheck struct {
	Header      string
	Severity    Severity
	Title       string
	Remediation string
}

var requiredHeaders = []headerCheck{
	{
		Header:      "Strict-Transport-Security",
		Severity:    SeverityHigh,
		Title:       "Missing Strict-Transport-Security header",
		Remediation: "Add header: Strict-Transport-Security: max-age=31536000; includeSubDomains",
	},
	{
		Header:      "Content-Security-Policy",
		Severity:    SeverityMedium,
		Title:       "Missing Content-Security-Policy header",
		Remediation: "Add header: Content-Security-Policy: default-src 'self'",
	},
	{
		Header:      "X-Frame-Options",
		Severity:    SeverityMedium,
		Title:       "Missing X-Frame-Options header",
		Remediation: "Add header: X-Frame-Options: DENY (or SAMEORIGIN)",
	},
	{
		Header:      "X-Content-Type-Options",
		Severity:    SeverityLow,
		Title:       "Missing X-Content-Type-Options header",
		Remediation: "Add header: X-Content-Type-Options: nosniff",
	},
	{
		Header:      "Referrer-Policy",
		Severity:    SeverityLow,
		Title:       "Missing Referrer-Policy header",
		Remediation: "Add header: Referrer-Policy: strict-origin-when-cross-origin",
	},
	{
		Header:      "Permissions-Policy",
		Severity:    SeverityInfo,
		Title:       "Missing Permissions-Policy header",
		Remediation: "Add header: Permissions-Policy: geolocation=(), camera=()",
	},
}

// Scan checks HTTP response headers for security best practices.
func (h *HeaderScanner) Scan(ctx context.Context, target config.TargetConfig) ([]Finding, error) {
	var findings []Finding
	now := time.Now()

	for _, ep := range target.Endpoint {
		if ep.Protocol != "https" {
			continue
		}

		url := fmt.Sprintf("https://%s:%d", target.Host, ep.Port)

		client := &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()

		for _, check := range requiredHeaders {
			if resp.Header.Get(check.Header) == "" {
				findings = append(findings, Finding{
					Timestamp:   now,
					Target:      target.Name,
					Scanner:     "headers",
					Title:       fmt.Sprintf("%s on %s", check.Title, url),
					Description: fmt.Sprintf("The response from %s is missing the %s header.", url, check.Header),
					Severity:    check.Severity,
					Remediation: check.Remediation,
				})
			}
		}
	}

	return findings, nil
}
