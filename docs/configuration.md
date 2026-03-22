# Configuration Reference

All configuration is in `config/homelab.yaml` (or `/etc/homelab-dash/homelab.yaml` for manual install).

## Complete Example

```yaml
server:
  host: "0.0.0.0"           # Listen address
  port: 8080                 # Listen port
  auth:
    enabled: true            # Enable HTTP Basic Auth
    username: "admin"
    password: "changeme"

defaults:
  interval: "60s"            # Service check interval
  scan_interval: "6h"        # Security scan interval
  alert_interval: "60s"      # Alert evaluation interval

targets:
  - name: "My Service"       # Display name
    host: "192.168.1.100"    # IP or hostname
    checks: [ping, http]     # Check types: ping, http
    ports: [8080]            # Ports to check/scan

  - name: "HTTPS Service"
    host: "192.168.1.101"
    checks: [ping, http]
    ports: ["https:443"]     # Explicit protocol

  - name: "Multi-port"
    host: "192.168.1.102"
    checks: [ping, http]
    ports: [80, 443, 8080]

integrations:
  proxmox:
    enabled: false
    hosts:                      # Multiple hosts for redundancy
      - "https://10.0.0.200:8006"
      - "https://10.0.0.204:8006"
    token_id: "user@pam!token"
    token_secret: "your-secret"
    insecure_tls: true          # Skip TLS verify (for self-signed)
    nodes: ["pve", "node2"]     # Node names to monitor

  jellyfin:
    enabled: false
    host: "http://192.168.1.30:8096"
    api_key: "your-api-key"

  frigate:
    enabled: false
    host: "http://192.168.1.31:5000"

  nas:
    enabled: false
    smart_devices: ["/dev/sda", "/dev/sdb"]

alerts:
  - name: "Service down"
    condition: "status == down"
    target: ""                  # Empty = all targets
    duration: "5m"              # How long before firing
    webhook: "http://ntfy/homelab"
    severity: "critical"

  - name: "High CPU"
    condition: "metric > threshold"
    target: "node:prox2"
    metric: "node.cpu.percent"
    threshold: 85
    duration: "10m"
    webhook: "http://ntfy/homelab"
    severity: "warn"
```

## Field Reference

### server

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| host | string | "0.0.0.0" | Listen address |
| port | int | 8080 | Listen port |
| auth.enabled | bool | false | Enable HTTP Basic Auth |
| auth.username | string | - | Auth username |
| auth.password | string | - | Auth password |

### defaults

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| interval | string | "60s" | Service check interval |
| scan_interval | string | "6h" | Security scan interval |
| alert_interval | string | "60s" | Alert evaluation interval |

### targets

| Field | Type | Description |
|-------|------|-------------|
| name | string | Display name (must be unique) |
| host | string | IP address or hostname |
| checks | []string | Check types: "ping", "http" |
| ports | []interface{} | Ports as int or "protocol:port" string |

### integrations.proxmox

| Field | Type | Description |
|-------|------|-------------|
| enabled | bool | Enable Proxmox integration |
| hosts | []string | Proxmox API URLs (for redundancy) |
| token_id | string | API token ID |
| token_secret | string | API token secret |
| insecure_tls | bool | Skip TLS certificate verification |
| nodes | []string | Proxmox node names to monitor |

### integrations.jellyfin

| Field | Type | Description |
|-------|------|-------------|
| enabled | bool | Enable Jellyfin integration |
| host | string | Jellyfin server URL |
| api_key | string | Jellyfin API key |

### integrations.frigate

| Field | Type | Description |
|-------|------|-------------|
| enabled | bool | Enable Frigate integration |
| host | string | Frigate server URL |

### alerts

| Field | Type | Description |
|-------|------|-------------|
| name | string | Alert rule name |
| condition | string | "status == down", "status == degraded", "metric > threshold", "vm.status == stopped" |
| target | string | Target name (empty = all) |
| metric | string | Metric name (for threshold condition) |
| threshold | float64 | Threshold value |
| duration | string | Time condition must hold before firing |
| webhook | string | Webhook URL |
| severity | string | "info", "warn", "critical" |
