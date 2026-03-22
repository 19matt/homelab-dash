# homelab-dash

A self-hosted monitoring dashboard for homelab environments. Single binary, zero dependencies, pure Go.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/homelab/homelab-dash)](https://goreportcard.com/report/github.com/homelab/homelab-dash)

## Features

- **Service monitoring** — HTTP and ping checks for all your services
- **Proxmox integration** — VM/container status, node CPU/RAM metrics
- **Jellyfin integration** — Active sessions, library counts, transcoding status
- **Frigate integration** — Camera FPS, detector inference speed, recording stats
- **Security scanning** — Port scans, TLS certificate checks, HTTP header analysis
- **Alert engine** — Rule-based alerts with webhook dispatch (ntfy, Discord, Slack)
- **90-day uptime sparklines** — Visual uptime history per service
- **Dark/light theme** — Toggle with localStorage persistence
- **Settings page** — Add/remove targets without editing YAML
- **Audit log** — Track check failures, recoveries, and config changes

## Quick Start

### Docker Compose (recommended)

```bash
git clone https://github.com/homelab/homelab-dash.git
cd homelab-dash
docker compose up -d
```

Dashboard: http://localhost:8080

### Proxmox LXC

```bash
# One command to create a Debian LXC with homelab-dash pre-installed
curl -sSL https://raw.githubusercontent.com/homelab/homelab-dash/main/scripts/proxmox-lxc-template.sh | bash
```

### Manual Installation

```bash
curl -sSL https://raw.githubusercontent.com/homelab/homelab-dash/main/scripts/install.sh | sudo bash
```

## Configuration

Edit `config/homelab.yaml` (or `/etc/homelab-dash/homelab.yaml` for manual install):

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  auth:
    enabled: true
    username: "admin"
    password: "changeme"  # Change this!

defaults:
  interval: "60s"         # Service check interval
  scan_interval: "6h"     # Security scan interval
  alert_interval: "60s"   # Alert evaluation interval

targets:
  - name: "My Service"
    host: "192.168.1.100"
    checks: [ping, http]
    ports: [8080]

integrations:
  proxmox:
    enabled: true
    hosts:
      - "https://10.0.0.200:8006"
    token_id: "user@pam!token"
    token_secret: "your-token-secret"
    insecure_tls: true
    nodes: ["pve"]

  jellyfin:
    enabled: true
    host: "http://10.0.0.166:8096"
    api_key: "your-api-key"

  frigate:
    enabled: true
    host: "http://10.0.0.96:5000"

alerts:
  - name: "Service down"
    condition: "status == down"
    duration: "5m"
    webhook: "http://ntfy-server/homelab-alerts"
    severity: "critical"
```

See [docs/configuration.md](docs/configuration.md) for the full reference.

## Documentation

- [Installation Guide](docs/installation.md) — Docker, LXC, manual
- [Configuration Reference](docs/configuration.md) — All YAML options
- [Proxmox Setup](docs/proxmox-setup.md) — Creating API tokens
- [Jellyfin Setup](docs/jellyfin-setup.md) — Finding your API key
- [API Reference](docs/api.md) — REST endpoints
- [Development](docs/development.md) — Building from source
- [Changelog](CHANGELOG.md)

## Architecture

```
homelab-dash/
├── cmd/server/main.go          # Entry point
├── internal/
│   ├── checker/                # Health check implementations
│   ├── collector/              # Metric collector interface
│   ├── config/                 # YAML config loading
│   ├── integration/            # Proxmox, Jellyfin, Frigate adapters
│   ├── scanner/                # Security scanning
│   ├── scheduler/              # Periodic task orchestration
│   ├── store/                  # SQLite storage + migrations
│   └── web/                    # HTTP server, handlers, templates
├── scripts/                    # LXC template, install script
├── Dockerfile
└── docker-compose.yml
```

## Upgrade

### Docker
```bash
docker compose pull && docker compose up -d
```

### LXC/Manual
```bash
curl -sSL https://raw.githubusercontent.com/homelab/homelab-dash/main/scripts/install.sh | sudo bash
```

Database migrations run automatically on startup.

## License

MIT
