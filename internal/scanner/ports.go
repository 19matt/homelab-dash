package scanner

import (
	"context"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/homelab/homelab-dash/internal/config"
)

// commonPorts are ports commonly scanned by the port scanner.
var commonPorts = []int{
	21, 22, 23, 25, 53, 80, 110, 143, 443, 445,
	993, 995, 3306, 3389, 5432, 5900, 6379,
	8006, 8080, 8443, 9090,
}

// PortScanner scans for open TCP ports on a target.
type PortScanner struct{}

// Name returns the scanner name.
func (p *PortScanner) Name() string {
	return "ports"
}

// Scan performs a port scan against the target.
func (p *PortScanner) Scan(ctx context.Context, target config.TargetConfig) ([]Finding, error) {
	// Build set of known ports from config
	knownPorts := make(map[int]bool)
	for _, ep := range target.Endpoint {
		knownPorts[ep.Port] = true
	}

	// Build port list: common ports + known ports
	portSet := make(map[int]bool)
	for _, port := range commonPorts {
		portSet[port] = true
	}
	for port := range knownPorts {
		portSet[port] = true
	}

	var ports []int
	for port := range portSet {
		ports = append(ports, port)
	}
	sort.Ints(ports)

	// Worker pool with semaphore
	sem := make(chan struct{}, 50)
	var mu sync.Mutex
	var openPorts []int

	var wg sync.WaitGroup
	for _, port := range ports {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			addr := net.JoinHostPort(target.Host, fmt.Sprintf("%d", port))
			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err == nil {
				conn.Close()
				mu.Lock()
				openPorts = append(openPorts, port)
				mu.Unlock()
			}
		}(port)
	}
	wg.Wait()

	sort.Ints(openPorts)

	var findings []Finding
	now := time.Now()

	// Info finding: open ports found
	if len(openPorts) > 0 {
		var portList string
		for _, port := range openPorts {
			if portList != "" {
				portList += ", "
			}
			portList += fmt.Sprintf("%d", port)
		}
		findings = append(findings, Finding{
			Timestamp:   now,
			Target:      target.Name,
			Scanner:     "ports",
			Title:       fmt.Sprintf("Open ports found on %s", target.Name),
			Description: fmt.Sprintf("The following TCP ports are open: %s", portList),
			Severity:    SeverityInfo,
			Remediation: "Review open ports and close any that are not required.",
		})
	}

	// Medium finding: unexpected ports
	for _, port := range openPorts {
		if !knownPorts[port] {
			findings = append(findings, Finding{
				Timestamp:   now,
				Target:      target.Name,
				Scanner:     "ports",
				Title:       fmt.Sprintf("Unexpected port %d open on %s", port, target.Name),
				Description: fmt.Sprintf("Port %d is open but not listed in the target configuration.", port),
				Severity:    SeverityMedium,
				Remediation: fmt.Sprintf("Verify port %d is intentional. If not, close it via firewall or service configuration.", port),
			})
		}
	}

	return findings, nil
}
