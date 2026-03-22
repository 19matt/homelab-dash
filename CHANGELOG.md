# Changelog

## v0.1.0 (2026-03-22)

Initial release.

### Features

**Core Monitoring**
- Service health checks (ping, HTTP)
- Configurable check intervals
- Real-time status updates via SSE

**Proxmox Integration**
- Multi-node cluster support with host redundancy
- VM and container status monitoring
- Node CPU, memory, load metrics
- 90-day uptime sparklines per VM

**Jellyfin Integration**
- Server health check
- Active session monitoring
- Library statistics (movies, episodes, songs)
- Transcoding detection

**Frigate Integration**
- Server health check
- Per-camera FPS monitoring
- Detector inference speed tracking

**Security Scanning**
- Port scanning with worker pool
- TLS certificate analysis
- HTTP security header checks
- Findings history and resolution tracking

**Alerting**
- Rule-based alert engine
- Duration-based deduplication
- Webhook dispatch (ntfy, Discord, Slack compatible)
- Alert event history

**Dashboard**
- Dark and light themes with toggle
- Overview page with node resource bars
- Services page with 90-day uptime sparklines
- Proxmox page with VM table and gauges
- Metrics page with custom date range picker
- Security page with filtering and resolution tracking
- Host detail page with mini-charts
- Jellyfin and Frigate pages

**Administration**
- Settings page for add/remove targets
- Test connection per target
- Audit log with type filtering
- Admin status endpoint

**Infrastructure**
- Docker multi-stage build (< 30MB)
- Docker Compose with healthcheck
- Proxmox LXC template script
- Standalone installer for Debian/Ubuntu
- Versioned database migrations
- Data retention cleanup (90 days)
