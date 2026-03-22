# Development

## Prerequisites

- Go 1.22+
- SQLite (via modernc.org/sqlite, no CGO required)
- Docker (optional, for container builds)

## Running Locally

```bash
git clone https://github.com/homelab/homelab-dash.git
cd homelab-dash

# Run with default config
go run ./cmd/server --config config/homelab.yaml

# Run with custom config
go run ./cmd/server --config /path/to/config.yaml --db /tmp/test.db
```

## Building

```bash
# Simple build
go build -o homelab-dash ./cmd/server

# With version injection
go build -ldflags="-X main.Version=v0.1.0 -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o homelab-dash ./cmd/server

# Static binary (no CGO)
CGO_ENABLED=0 go build -o homelab-dash ./cmd/server
```

## Project Structure

```
homelab-dash/
├── cmd/server/main.go          # Entry point, CLI flags, wiring
├── internal/
│   ├── alert/                  # Alert engine + webhook dispatch
│   ├── checker/                # Health check implementations
│   ├── collector/              # Metric collector interface
│   ├── config/                 # YAML config loading + validation
│   ├── integration/            # Proxmox, Jellyfin, Frigate adapters
│   │   ├── proxmox/
│   │   ├── jellyfin/
│   │   └── frigate/
│   ├── scanner/                # Security scanning (ports, TLS, headers)
│   ├── scheduler/              # Periodic task orchestration
│   ├── store/                  # SQLite storage, migrations, queries
│   └── web/
│       ├── handlers/           # HTTP handlers
│       ├── static/             # CSS, JS
│       ├── templates/          # HTML templates
│       ├── hub.go              # SSE hub
│       ├── middleware.go       # Auth middleware
│       └── server.go           # Route registration
├── scripts/                    # LXC template, install script
├── docs/                       # Documentation
├── config/                     # Example configs
├── Dockerfile
└── docker-compose.yml
```

## Adding a New Integration

1. Create `internal/integration/myapp/`
2. Add client, collector, checker files
3. Add config struct to `internal/config/config.go`
4. Wire in `cmd/server/main.go`
5. Add page handler + template
6. Register routes in `internal/web/server.go`

## Adding a New Checker

1. Create `internal/checker/mychecker.go`
2. Implement the `Checker` interface:
   ```go
   type Checker interface {
       Name() string
       Check(ctx context.Context) (CheckResult, error)
   }
   ```
3. Register in main.go or via config

## Database Migrations

Add new migrations to `internal/store/migrations.go`:

```go
var Migrations = []Migration{
    {Version: "001", SQL: `...`},
    {Version: "002", SQL: `...`},
    {Version: "003", SQL: `YOUR_NEW_MIGRATION`},  // Add here
}
```

Migrations run automatically on startup.

## Testing

```bash
go build ./...
go vet ./...
```

For integration testing, start the server and hit the endpoints:

```bash
go run ./cmd/server --config config/homelab.yaml &
curl http://localhost:8080/health
```

## Code Style

- Follow standard Go conventions
- Use `context.Context` for all I/O operations
- Interfaces defined where they're consumed
- Errors returned, never panicked
- All config validated at startup
