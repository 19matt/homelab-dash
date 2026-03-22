package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/homelab/homelab-dash/internal/config"
)

// Client is a thin HTTP client for the Proxmox REST API.
type Client struct {
	host        string
	tokenID     string
	tokenSecret string
	httpClient  *http.Client
}

// NewClient creates a Proxmox API client from config.
func NewClient(cfg config.ProxmoxConfig) *Client {
	transport := &http.Transport{}
	if cfg.InsecureTLS {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	return &Client{
		host:        cfg.Host,
		tokenID:     cfg.TokenID,
		tokenSecret: cfg.TokenSecret,
		httpClient: &http.Client{
			Transport: transport,
		},
	}
}

// get performs an authenticated GET request and decodes the data field into result.
func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	url := c.host + "/api2/json" + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("proxmox: create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("proxmox: request %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("proxmox: request %s: status %d", path, resp.StatusCode)
	}

	var wrapper proxmoxResponse
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return fmt.Errorf("proxmox: decode response: %w", err)
	}

	if err := json.Unmarshal(wrapper.Data, result); err != nil {
		return fmt.Errorf("proxmox: unmarshal data: %w", err)
	}

	return nil
}

// GetNodeStatus returns the current status of a Proxmox node.
func (c *Client) GetNodeStatus(ctx context.Context, node string) (NodeStatus, error) {
	var status NodeStatus
	err := c.get(ctx, fmt.Sprintf("/nodes/%s/status", node), &status)
	return status, err
}

// GetNodeRRD returns RRD data for a node over the given timeframe.
func (c *Client) GetNodeRRD(ctx context.Context, node, timeframe string) ([]RRDEntry, error) {
	var entries []RRDEntry
	err := c.get(ctx, fmt.Sprintf("/nodes/%s/rrddata?timeframe=%s&cf=AVERAGE", node, timeframe), &entries)
	return entries, err
}

// ListVMs returns all QEMU VMs on a node.
func (c *Client) ListVMs(ctx context.Context, node string) ([]VMSummary, error) {
	var vms []VMSummary
	err := c.get(ctx, fmt.Sprintf("/nodes/%s/qemu", node), &vms)
	for i := range vms {
		vms[i].Type = "qemu"
	}
	return vms, err
}

// ListContainers returns all LXC containers on a node.
func (c *Client) ListContainers(ctx context.Context, node string) ([]VMSummary, error) {
	var containers []VMSummary
	err := c.get(ctx, fmt.Sprintf("/nodes/%s/lxc", node), &containers)
	for i := range containers {
		containers[i].Type = "lxc"
	}
	return containers, err
}

// GetVMStatus returns the current status of a specific VM or container.
func (c *Client) GetVMStatus(ctx context.Context, node string, vmid int, vmtype string) (VMSummary, error) {
	var status VMSummary
	err := c.get(ctx, fmt.Sprintf("/nodes/%s/%s/%d/status/current", node, vmtype, vmid), &status)
	status.Type = vmtype
	return status, err
}

// GetVMRRD returns RRD data for a VM over the given timeframe.
func (c *Client) GetVMRRD(ctx context.Context, node string, vmid int, vmtype, timeframe string) ([]RRDEntry, error) {
	var entries []RRDEntry
	err := c.get(ctx, fmt.Sprintf("/nodes/%s/%s/%d/rrddata?timeframe=%s&cf=AVERAGE", node, vmtype, vmid, timeframe), &entries)
	return entries, err
}
