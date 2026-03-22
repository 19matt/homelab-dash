# Installation Guide

## Docker Compose (Recommended)

```bash
git clone https://github.com/homelab/homelab-dash.git
cd homelab-dash
cp config/homelab.example.yaml config/homelab.yaml
# Edit config/homelab.yaml with your services and integrations
docker compose up -d
```

Dashboard: http://localhost:8080

### With S.M.A.R.T. monitoring

```bash
docker compose --profile smart up -d
```

This runs a privileged container with access to disk devices.

## Proxmox LXC

Creates a Debian 12 LXC container with homelab-dash pre-installed:

```bash
# On your Proxmox host
curl -sSL https://raw.githubusercontent.com/homelab/homelab-dash/main/scripts/proxmox-lxc-template.sh | bash
```

Options:
```bash
./proxmox-lxc-template.sh [CTID] [STORAGE] [VERSION]
# Example: ./proxmox-lxc-template.sh 201 local-lvm v0.1.0
```

Then:
1. Edit config: `pct exec 200 -- nano /etc/homelab-dash/homelab.yaml`
2. Start: `pct exec 200 -- systemctl start homelab-dash`

## Manual Installation

Works on any Debian/Ubuntu system:

```bash
curl -sSL https://raw.githubusercontent.com/homelab/homelab-dash/main/scripts/install.sh | sudo bash
```

This:
1. Downloads the latest binary to `/opt/homelab-dash/`
2. Creates config at `/etc/homelab-dash/homelab.yaml`
3. Installs a systemd service
4. Creates a `homelab-dash` user

### Start the service

```bash
sudo systemctl start homelab-dash
sudo systemctl status homelab-dash
```

### View logs

```bash
sudo journalctl -u homelab-dash -f
```

## Building from Source

Requires Go 1.22+:

```bash
git clone https://github.com/homelab/homelab-dash.git
cd homelab-dash
go build -o homelab-dash ./cmd/server
./homelab-dash --config config/homelab.yaml
```

With version injection:

```bash
go build -ldflags="-X main.Version=v0.1.0 -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o homelab-dash ./cmd/server
```

## Upgrading

### Docker
```bash
docker compose pull
docker compose up -d
```

### LXC/Manual
```bash
curl -sSL https://raw.githubusercontent.com/homelab/homelab-dash/main/scripts/install.sh | sudo bash
```

Database migrations run automatically on startup.
