# API Reference

All endpoints require authentication if `server.auth.enabled` is true (except `/health`).

## Health

- `GET /health` — Returns `homelab-dash ok` (public, no auth)

## Status

- `GET /api/status` — Latest check results per target+check
- `GET /api/uptime?target=X&check=Y&window=24h` — Uptime percentage
- `GET /api/uptime/daily?target=X&check=Y&days=90` — Daily uptime for sparklines

## Metrics

- `GET /api/metrics/latest?target=X` — Latest metric values for a target
- `GET /api/metrics/history?target=X&metric=Y&window=1h` — Time series data
- `GET /api/metrics/history?target=X&metric=Y&from=RFC3339&to=RFC3339` — Custom range
- `GET /api/metrics/targets` — All targets with metric data

## Proxmox

- `GET /api/proxmox/vms` — All VMs and containers across nodes

## Security

- `GET /api/security/findings?target=X&severity=high` — Filtered findings
- `GET /api/security/summary` — Severity counts
- `GET /api/security/report` — Download JSON report (last 30 days)
- `GET /api/security/resolved` — Resolved findings history

## Alerts

- `GET /api/alerts/events?since=RFC3339&limit=100` — Alert event history

## Audit

- `GET /api/audit/events?type=checks&page=1` — Paginated audit events

## Admin

- `GET /admin/status` — System status (version, uptime, DB size, table counts)

## Settings

- `POST /api/targets` — Add a target
- `DELETE /api/targets/{name}` — Remove a target
- `POST /api/targets/test` — Test connection to a target

## Server-Sent Events

- `GET /events` — SSE stream for real-time check result updates

Event format:
```
data: {"target":"My Service","check":"ping","status":"pass","latency_ms":4}
```

## Htmx Fragments

- `GET /fragments/status-grid` — Service status cards
- `GET /fragments/proxmox-summary` — Proxmox node gauges
- `GET /fragments/vm-table` — VM table
- `GET /fragments/jellyfin-summary` — Jellyfin widget
- `GET /fragments/frigate-summary` — Frigate widget
- `GET /fragments/security-summary` — Security findings summary
- `GET /fragments/findings-badge` — Nav findings badge
