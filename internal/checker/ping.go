package checker

import (
	"context"
	"fmt"
	"net"
	"time"
)

// PingChecker probes TCP connectivity to a host:port as a connectivity proxy.
type PingChecker struct {
	target  string
	host    string
	port    int
	timeout time.Duration
}

// NewPingChecker creates a PingChecker for the given target.
func NewPingChecker(target, host string, port int) *PingChecker {
	return &PingChecker{
		target:  target,
		host:    host,
		port:    port,
		timeout: 3 * time.Second,
	}
}

// Name returns the check name.
func (p *PingChecker) Name() string {
	return "ping"
}

// Check performs a TCP dial to verify connectivity.
func (p *PingChecker) Check(ctx context.Context) (CheckResult, error) {
	addr := net.JoinHostPort(p.host, fmt.Sprintf("%d", p.port))

	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, p.timeout)
	latency := time.Since(start)

	result := CheckResult{
		Timestamp: time.Now(),
		Target:    p.target,
		Check:     "ping",
		Latency:   latency,
	}

	if err != nil {
		result.Status = StatusFail
		result.Message = fmt.Sprintf("connection to %s failed: %v", addr, err)
		return result, nil
	}
	conn.Close()

	result.Status = StatusPass
	result.Message = "reachable"
	return result, nil
}
