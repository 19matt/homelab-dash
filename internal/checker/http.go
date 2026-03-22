package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

// HTTPChecker performs an HTTP request and validates the response status code.
type HTTPChecker struct {
	target   string
	url      string
	expected int
	timeout  time.Duration
}

// NewHTTPChecker creates an HTTPChecker for the given target and scheme/port.
func NewHTTPChecker(target, host string, port int, scheme string) *HTTPChecker {
	url := fmt.Sprintf("%s://%s:%d", scheme, host, port)
	return &HTTPChecker{
		target:   target,
		url:      url,
		expected: 200,
		timeout:  10 * time.Second,
	}
}

// Name returns the check name including the port.
func (h *HTTPChecker) Name() string {
	return fmt.Sprintf("http:%d", h.port())
}

func (h *HTTPChecker) port() int {
	// Extract port from URL for naming
	var scheme, host string
	var port int
	fmt.Sscanf(h.url, "%[^:]://%[^:]:%d", &scheme, &host, &port)
	if port == 0 {
		if scheme == "https" {
			port = 443
		} else {
			port = 80
		}
	}
	return port
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
