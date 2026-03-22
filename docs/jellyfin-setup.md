# Jellyfin Setup

## Finding Your API Key

1. Log into Jellyfin at `http://your-jellyfin:8096`
2. Click your profile icon (top right) → **Administration** → **Dashboard**
3. Go to **Advanced** → **API Keys**
4. Click **+ New API Key**
5. Enter a name (e.g., "homelab-dash") and click **OK**
6. Copy the generated key

## Configuration

```yaml
integrations:
  jellyfin:
    enabled: true
    host: "http://10.0.0.166:8096"
    api_key: "your-api-key-here"
```

## What's Monitored

- **Server health**: Reachable, version, server name
- **Active sessions**: Users currently streaming
- **Transcoding**: Count of sessions transcoding
- **Library**: Movie, series, episode, and song counts

## Testing

```bash
# Test API access
curl -H "X-Emby-Token: your-api-key" http://10.0.0.166:8096/System/Info
```

Should return JSON with server name and version.
