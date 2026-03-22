package scanner

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"time"

	"github.com/homelab/homelab-dash/internal/config"
)

// TLSScanner inspects TLS certificates on HTTPS endpoints.
type TLSScanner struct{}

// Name returns the scanner name.
func (t *TLSScanner) Name() string {
	return "tls"
}

// Scan checks TLS certificates on HTTPS endpoints.
func (t *TLSScanner) Scan(ctx context.Context, target config.TargetConfig) ([]Finding, error) {
	var findings []Finding
	now := time.Now()

	for _, ep := range target.Endpoint {
		if ep.Protocol != "https" {
			continue
		}

		addr := fmt.Sprintf("%s:%d", target.Host, ep.Port)
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second},
			"tcp", addr, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			findings = append(findings, Finding{
				Timestamp:   now,
				Target:      target.Name,
				Scanner:     "tls",
				Title:       fmt.Sprintf("TLS connection failed on %s:%d", target.Host, ep.Port),
				Description: fmt.Sprintf("Could not establish TLS connection: %v", err),
				Severity:    SeverityHigh,
				Remediation: "Verify the TLS certificate is properly configured on the server.",
			})
			continue
		}
		defer conn.Close()

		state := conn.ConnectionState()
		if len(state.PeerCertificates) == 0 {
			continue
		}

		cert := state.PeerCertificates[0]

		// Check TLS version
		if state.Version < tls.VersionTLS12 {
			findings = append(findings, Finding{
				Timestamp:   now,
				Target:      target.Name,
				Scanner:     "tls",
				Title:       fmt.Sprintf("Outdated TLS version on %s:%d", target.Host, ep.Port),
				Description: fmt.Sprintf("Server negotiated TLS %s, which is below TLS 1.2.", tlsVersionName(state.Version)),
				Severity:    SeverityMedium,
				Remediation: "Configure the server to require TLS 1.2 or higher.",
			})
		}

		// Check cert expiry
		daysUntilExpiry := time.Until(cert.NotAfter).Hours() / 24
		switch {
		case time.Now().After(cert.NotAfter):
			findings = append(findings, Finding{
				Timestamp:   now,
				Target:      target.Name,
				Scanner:     "tls",
				Title:       fmt.Sprintf("Expired TLS certificate on %s:%d", target.Host, ep.Port),
				Description: fmt.Sprintf("Certificate for %s expired on %s.", cert.Subject.CommonName, cert.NotAfter.Format("2006-01-02")),
				Severity:    SeverityCritical,
				Remediation: "Renew the TLS certificate immediately.",
			})
		case daysUntilExpiry <= 30:
			findings = append(findings, Finding{
				Timestamp:   now,
				Target:      target.Name,
				Scanner:     "tls",
				Title:       fmt.Sprintf("TLS certificate expiring soon on %s:%d", target.Host, ep.Port),
				Description: fmt.Sprintf("Certificate for %s expires in %.0f days.", cert.Subject.CommonName, daysUntilExpiry),
				Severity:    SeverityHigh,
				Remediation: "Plan to renew the TLS certificate before it expires.",
			})
		}

		// Check SHA-1 signature
		if cert.SignatureAlgorithm == x509.SHA1WithRSA || cert.SignatureAlgorithm == x509.ECDSAWithSHA1 {
			findings = append(findings, Finding{
				Timestamp:   now,
				Target:      target.Name,
				Scanner:     "tls",
				Title:       fmt.Sprintf("SHA-1 certificate on %s:%d", target.Host, ep.Port),
				Description: "The certificate uses SHA-1, which is deprecated and insecure.",
				Severity:    SeverityLow,
				Remediation: "Reissue the certificate using SHA-256 or stronger.",
			})
		}

		// Info: cert details
		var sans string
		for _, s := range cert.DNSNames {
			if sans != "" {
				sans += ", "
			}
			sans += s
		}
		findings = append(findings, Finding{
			Timestamp: now,
			Target:    target.Name,
			Scanner:   "tls",
			Title:     fmt.Sprintf("TLS certificate info for %s:%d", target.Host, ep.Port),
			Description: fmt.Sprintf("Subject: %s | Issuer: %s | Expires: %s | SANs: %s | TLS: %s",
				cert.Subject.CommonName, cert.Issuer.CommonName,
				cert.NotAfter.Format("2006-01-02"), sans, tlsVersionName(state.Version)),
			Severity:    SeverityInfo,
			Remediation: "",
		})
	}

	return findings, nil
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("unknown (%d)", v)
	}
}
