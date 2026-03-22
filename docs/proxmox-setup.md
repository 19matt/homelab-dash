# Proxmox Setup

## Creating an API Token

1. Log into Proxmox web UI at `https://your-proxmox:8006`
2. Go to **Datacenter → Permissions → API Tokens**
3. Click **Add**
4. Settings:
   - **User**: Create a new user (e.g., `homelab-dash@pam`) or use existing
   - **Token ID**: `monitoring`
   - **Privilege Separation**: Uncheck this to inherit user permissions
5. Click **Add** and copy the **Secret** (shown only once!)

## Required Permissions

The token needs **PVEAuditor** role for read-only monitoring:

```bash
# On Proxmox host
pveum acl modify / --token 'homelab-dash@pam!monitoring' --roles PVEAuditor
```

For specific node access only:
```bash
pveum acl modify /nodes/pve --token 'homelab-dash@pam!monitoring' --roles PVEAuditor
```

## Multi-Node Cluster

In a Proxmox cluster, connecting to any single node provides access to all nodes. Use `hosts` for redundancy:

```yaml
integrations:
  proxmox:
    enabled: true
    hosts:
      - "https://10.0.0.200:8006"
      - "https://10.0.0.201:8006"
      - "https://10.0.0.202:8006"
    token_id: "homelab-dash@pam!monitoring"
    token_secret: "your-secret"
    insecure_tls: true
    nodes: ["pve1", "pve2", "pve3"]
```

## Self-Signed Certificates

If Proxmox uses self-signed certificates (default), set `insecure_tls: true`.

## Testing

```bash
# Test API access from your machine
curl -sk -H "Authorization: PVEAPIToken=homelab-dash@pam!monitoring=your-secret" \
  https://10.0.0.200:8006/api2/json/nodes
```

Should return a JSON list of your nodes.
