#!/bin/bash
set -e

# homelab-dash Proxmox LXC Template Creator
# Creates a Debian 12 LXC container with homelab-dash pre-installed
#
# Usage:
#   ./proxmox-lxc-template.sh [CTID] [STORAGE] [VERSION]
#
# Examples:
#   ./proxmox-lxc-template.sh                    # CTID=200, STORAGE=local, VERSION=latest
#   ./proxmox-lxc-template.sh 201 local-lvm v0.1.0

CTID=${1:-200}
STORAGE=${2:-local-lvm}
VERSION=${3:-v0.1.0}

# Auto-detect template storage (needs vztmpl support)
TEMPLATE_STORAGE="local"
if ! pvesm status | grep -q "^${TEMPLATE_STORAGE}.*dir"; then
    # Try to find any dir-based storage
    TEMPLATE_STORAGE=$(pvesm status | awk '$2 == "dir" {print $1}' | head -1)
    if [ -z "$TEMPLATE_STORAGE" ]; then
        TEMPLATE_STORAGE="local"
    fi
fi

echo "=== homelab-dash Proxmox LXC Setup ==="
echo "CTID:             $CTID"
echo "Rootfs Storage:   $STORAGE"
echo "Template Storage: $TEMPLATE_STORAGE"
echo "Version:          $VERSION"
echo ""

# Find the latest Debian 12 template
TEMPLATE=$(pveam available --section system | grep debian-12 | tail -1 | awk '{print $2}')
if [ -z "$TEMPLATE" ]; then
    echo "Downloading Debian 12 template..."
    pveam update
    TEMPLATE=$(pveam available --section system | grep debian-12 | tail -1 | awk '{print $2}')
fi

if [ -z "$TEMPLATE" ]; then
    echo "ERROR: Could not find Debian 12 template"
    echo "Available templates:"
    pveam available --section system
    exit 1
fi

echo "Using template: $TEMPLATE"

# Check if template is downloaded
if ! pveam list $TEMPLATE_STORAGE | grep -q "$TEMPLATE"; then
    echo "Downloading template to $TEMPLATE_STORAGE..."
    pveam download $TEMPLATE_STORAGE $TEMPLATE
fi

# Check if CTID already exists
if pct status $CTID &>/dev/null; then
    echo "ERROR: Container $CTID already exists"
    exit 1
fi

# Get host network info (use first bridge)
BRIDGE=$(ip route | grep default | awk '{print $5}' | head -1)
if [ -z "$BRIDGE" ]; then
    BRIDGE="vmbr0"
fi

# Get next available IP (DHCP by default, but user can change)
IP="dhcp"

echo ""
echo "Creating LXC container..."
pct create $CTID ${TEMPLATE_STORAGE}:vztmpl/${TEMPLATE} \
    --hostname homelab-dash \
    --memory 512 \
    --cores 1 \
    --rootfs ${STORAGE}:2 \
    --net0 name=eth0,bridge=${BRIDGE},ip=${IP} \
    --unprivileged 0 \
    --onboot 1 \
    --description "homelab-dash monitoring dashboard"

echo "Starting container..."
pct start $CTID

# Wait for container to be ready
echo "Waiting for container to boot..."
sleep 10

echo "Installing dependencies..."
pct exec $CTID -- bash -c "
    apt-get update -qq
    apt-get install -y -qq curl wget
"

echo "Installing homelab-dash..."
pct exec $CTID -- bash -c "
    # Create directories
    mkdir -p /opt/homelab-dash /etc/homelab-dash /var/lib/homelab-dash

    # Download binary
    curl -sL -o /opt/homelab-dash/homelab-dash \
        https://github.com/homelab/homelab-dash/releases/download/${VERSION}/homelab-dash-linux-amd64

    # If latest, use latest release URL
    if [ '${VERSION}' = 'latest' ]; then
        curl -sL -o /opt/homelab-dash/homelab-dash \
            https://github.com/homelab/homelab-dash/releases/latest/download/homelab-dash-linux-amd64
    fi

    chmod +x /opt/homelab-dash/homelab-dash

    # Create systemd service
    cat > /etc/systemd/system/homelab-dash.service << 'SERVICE'
[Unit]
Description=homelab-dash monitoring dashboard
After=network.target
Wants=network.target

[Service]
Type=simple
ExecStart=/opt/homelab-dash/homelab-dash --config /etc/homelab-dash/homelab.yaml --db /var/lib/homelab-dash/homelab.db
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
SERVICE

    systemctl daemon-reload
    systemctl enable homelab-dash
"

echo "Creating default config..."
pct exec $CTID -- bash -c 'cat > /etc/homelab-dash/homelab.yaml << "EOF"
server:
  host: "0.0.0.0"
  port: 8080
  auth:
    enabled: true
    username: "admin"
    password: "changeme"

defaults:
  interval: "60s"
  scan_interval: "6h"
  alert_interval: "60s"

targets: []

integrations:
  proxmox:
    enabled: false
    hosts: []
    token_id: ""
    token_secret: ""
    insecure_tls: true
    nodes: []
  jellyfin:
    enabled: false
    host: ""
    api_key: ""
  frigate:
    enabled: false
    host: ""
  nas:
    enabled: false
    smart_devices: []

alerts: []
EOF'

# Get container IP
CONTAINER_IP=$(pct exec $CTID -- hostname -I | awk '{print $1}')

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Container ID: $CTID"
echo "Container IP: ${CONTAINER_IP:-dhcp}"
echo ""
echo "Next steps:"
echo "1. Edit config:  pct exec $CTID -- nano /etc/homelab-dash/homelab.yaml"
echo "2. Start service: pct exec $CTID -- systemctl start homelab-dash"
echo "3. Check status:  pct exec $CTID -- systemctl status homelab-dash"
echo "4. View logs:     pct exec $CTID -- journalctl -u homelab-dash -f"
echo ""
if [ "$IP" = "dhcp" ]; then
    echo "Dashboard: http://${CONTAINER_IP:-<get-ip>}:8080"
else
    echo "Dashboard: http://${IP%%/*}:8080"
fi
echo ""
echo "To upgrade later:"
echo "  pct exec $CTID -- bash -c 'curl -sL https://github.com/homelab/homelab-dash/releases/latest/download/homelab-dash-linux-amd64 -o /opt/homelab-dash/homelab-dash && chmod +x /opt/homelab-dash/homelab-dash && systemctl restart homelab-dash'"
