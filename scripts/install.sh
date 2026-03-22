#!/bin/bash
set -e

# homelab-dash installer for Debian/Ubuntu systems
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/homelab/homelab-dash/main/scripts/install.sh | bash
#   curl -sSL https://raw.githubusercontent.com/homelab/homelab-dash/main/scripts/install.sh | bash -s v0.1.0

VERSION="${1:-latest}"
INSTALL_DIR="/opt/homelab-dash"
CONFIG_DIR="/etc/homelab-dash"
DATA_DIR="/var/lib/homelab-dash"

echo "=== homelab-dash installer ==="
echo "Version: $VERSION"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "ERROR: This script must be run as root"
    exit 1
fi

# Create user if not exists
if ! id -u homelab-dash &>/dev/null; then
    echo "Creating user homelab-dash..."
    useradd -r -s /usr/sbin/nologin -d $DATA_DIR homelab-dash
fi

# Create directories
echo "Creating directories..."
mkdir -p $INSTALL_DIR $CONFIG_DIR $DATA_DIR
chown homelab-dash:homelab-dash $DATA_DIR

# Download binary
echo "Downloading homelab-dash $VERSION..."
if [ "$VERSION" = "latest" ]; then
    URL="https://github.com/homelab/homelab-dash/releases/latest/download/homelab-dash-linux-amd64"
else
    URL="https://github.com/homelab/homelab-dash/releases/download/${VERSION}/homelab-dash-linux-amd64"
fi

curl -sL -o $INSTALL_DIR/homelab-dash $URL
chmod +x $INSTALL_DIR/homelab-dash

# Verify binary works
$INSTALL_DIR/homelab-dash --version

# Create default config if not exists
if [ ! -f $CONFIG_DIR/homelab.yaml ]; then
    echo "Creating default config..."
    cat > $CONFIG_DIR/homelab.yaml << 'EOF'
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
EOF
    echo "Default config created at $CONFIG_DIR/homelab.yaml"
else
    echo "Config already exists at $CONFIG_DIR/homelab.yaml"
fi

# Install systemd service
echo "Installing systemd service..."
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
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
SERVICE

systemctl daemon-reload
systemctl enable homelab-dash

# Get IP address
IP=$(hostname -I | awk '{print $1}')

echo ""
echo "=== Installation Complete ==="
echo ""
echo "1. Edit config:     nano $CONFIG_DIR/homelab.yaml"
echo "2. Start service:   systemctl start homelab-dash"
echo "3. Check status:    systemctl status homelab-dash"
echo "4. View logs:       journalctl -u homelab-dash -f"
echo ""
echo "Dashboard: http://${IP}:8080"
echo ""
echo "To upgrade:"
echo "  curl -sSL https://raw.githubusercontent.com/homelab/homelab-dash/main/scripts/install.sh | bash -s latest"
