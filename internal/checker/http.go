package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// HTTPChecker performs an HTTP request and validates the response status code.
type HTTPChecker struct {
	target   string
	url      string
	port     int
	expected int
	timeout  time.Duration
}

// NewHTTPChecker creates an HTTPChecker for the given target and scheme/port.
func NewHTTPChecker(target, host string, port int, scheme string) *HTTPChecker {
	return &HTTPChecker{
		target:   target,
		url:      fmt.Sprintf("%s://%s:%d", scheme, host, port),
		port:     port,
		expected: 200,
		timeout:  10 * time.Second,
	}
}

// Name returns the check name including the port.
func (h *HTTPChecker) Name() string {
	return fmt.Sprintf("http:%d", h.port)
}

// Check performs an HTTP request and evaluates the response.
func (h *HTTPChecker) Check(ctx context.Context) (CheckResult, error) {
	client := &http.Client{
		Timeout: h.timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", h.url, nil)
	if err != nil {
		return CheckResult{
			Timestamp: time.Now(),
			Target:    h.target,
			Check:     h.Name(),
			Status:    StatusFail,
			Message:   fmt.Sprintf("request creation failed: %v", err),
		}, nil
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	result := CheckResult{
		Timestamp: time.Now(),
		Target:    h.target,
		Check:     h.Name(),
		Latency:   latency,
	}

	if err != nil {
		result.Status = StatusFail
		result.Message = fmt.Sprintf("request to %s failed: %v", h.url, err)
		return result, nil
	}
	defer resp.Body.Close()

	result.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)

	switch {
	case resp.StatusCode == h.expected:
		result.Status = StatusPass
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		result.Status = StatusWarn
	case resp.StatusCode >= 500:
		result.Status = StatusFail
	default:
		result.Status = StatusPass
	}

	return result, nil
}

// ExtractPort extracts the port from a URL string, returning defaultPort if not found.
func ExtractPort(rawURL string, defaultPort int) int {
	u, err := url.Parse(rawURL)
	if err != nil {
		return defaultPort
	}
	if p, err := strconv.Atoi(u.Port()); err == nil {
		return p
	}
	return defaultPort
}
