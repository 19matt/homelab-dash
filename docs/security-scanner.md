# Security Scanner

The security scanner runs against configured targets on a scheduled interval (default 6h). Three scanners check for common vulnerabilities and misconfigurations.

## Port Scanner (`ports`)

Scans ~20 common ports (21, 22, 23, 25, 53, 80, 110, 143, 443, 445, 993, 995, 3306, 3389, 5432, 5900, 6379, 8006, 8080, 8443, 9090) plus any ports listed in the target config.

| Severity | Trigger |
|----------|---------|
| Info | Open ports found (summary list) |
| Medium | Port open that's NOT in the target config (unexpected) |

## TLS Scanner (`tls`)

Only runs on HTTPS endpoints. Inspects the certificate chain.

| Severity | Trigger | Example |
|----------|---------|---------|
| Critical | Certificate already expired | Cert expired 2025-01-01 |
| High | Certificate expires within 30 days | Expires in 12 days |
| Medium | TLS version < 1.2 negotiated | Server using TLS 1.0 or 1.1 |
| Low | Certificate uses SHA-1 signature | Deprecated algorithm |
| Info | Certificate details | Subject, issuer, expiry, SANs, TLS version |

## Header Scanner (`headers`)

Only runs on HTTPS endpoints. Checks for missing security response headers.

| Severity | Missing Header | Recommended Value |
|----------|---------------|-------------------|
| High | `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` |
| Medium | `Content-Security-Policy` | `default-src 'self'` |
| Medium | `X-Frame-Options` | `DENY` or `SAMEORIGIN` |
| Low | `X-Content-Type-Options` | `nosniff` |
| Low | `Referrer-Policy` | `strict-origin-when-cross-origin` |
| Info | `Permissions-Policy` | `geolocation=(), camera=()` |

## Configuration

Add to `homelab.yaml`:

```yaml
defaults:
  scan_interval: "6h"  # how often scans run
```

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /api/security/findings` | Filtered findings JSON. Supports `?target=X&severity=high&since=RFC3339` |
| `GET /api/security/summary` | Severity count map: `{"critical":0,"high":2,...}` |
| `GET /api/security/report` | Downloadable JSON file of last 30 days |

## Dashboard

The Security tab shows:
- Summary bar with severity counts
- Findings table sorted by severity (descending)
- Clickable rows to expand description and remediation
- Download report button
