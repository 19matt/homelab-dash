package handlers

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/homelab/homelab-dash/internal/integration/proxmox"
	"github.com/homelab/homelab-dash/internal/store"
)

// StatusGridFragment returns the service status cards for htmx refresh.
func StatusGridFragment(tmpl *template.Template, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		results, err := s.GetLatestCheckResults(r.Context())
		if err != nil {
			fragmentError(w, err, "Failed to load status")
			return
		}

		w.Header().Set("Content-Type", "text/html")
		tmpl.ExecuteTemplate(w, "fragment-status-grid", struct {
			StatusResults interface{}
		}{StatusResults: results})
	}
}

// ProxmoxSummaryFragment returns the proxmox summary for htmx refresh.
func ProxmoxSummaryFragment(tmpl *template.Template, s *store.Store, vmCollectors []*proxmox.VMCollector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var allVMs []proxmox.VMSummary
		for _, c := range vmCollectors {
			allVMs = append(allVMs, c.LatestVMs()...)
		}

		nodes := getNodeStats(s)

		data := struct {
			Nodes []NodeStat
			VMs   []proxmox.VMSummary
		}{Nodes: nodes, VMs: allVMs}

		w.Header().Set("Content-Type", "text/html")
		tmpl.ExecuteTemplate(w, "fragment-proxmox-summary", data)
	}
}

// VMTableFragment returns the VM table for htmx refresh.
func VMTableFragment(tmpl *template.Template, vmCollectors []*proxmox.VMCollector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var allVMs []proxmox.VMSummary
		for _, c := range vmCollectors {
			allVMs = append(allVMs, c.LatestVMs()...)
		}

		data := struct {
			VMs []proxmox.VMSummary
		}{VMs: allVMs}

		w.Header().Set("Content-Type", "text/html")
		tmpl.ExecuteTemplate(w, "fragment-vm-table", data)
	}
}

// FindingsBadgeFragment returns the findings count badge for the nav.
func FindingsBadgeFragment(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		summary, err := s.GetFindingsSummary(r.Context())
		if err != nil {
			fragmentError(w, err, "Failed to load findings")
			return
		}

		critical := summary["critical"] + summary["high"]

		w.Header().Set("Content-Type", "text/html")
		if critical > 0 {
			fmt.Fprintf(w, `<span class="badge badge-critical">%d</span>`, critical)
		} else {
			total := 0
			for _, c := range summary {
				total += c
			}
			if total > 0 {
				fmt.Fprintf(w, `<span class="badge badge-unknown">%d</span>`, total)
			} else {
				fmt.Fprint(w, `<span class="badge badge-unknown">0</span>`)
			}
		}
	}
}

// JellyfinSummaryFragment returns the Jellyfin summary widget.
func JellyfinSummaryFragment(client JellyfinClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		info, infoErr := client.GetSystemInfo(ctx)
		if infoErr != nil {
			fragmentError(w, infoErr, "Jellyfin unreachable")
			return
		}

		sessions, _ := client.GetSessions(ctx)
		counts, _ := client.GetItemCounts(ctx)

		w.Header().Set("Content-Type", "text/html")

		if infoErr != nil {
			fmt.Fprint(w, `<p class="muted">Jellyfin unreachable</p>`)
			return
		}

		active := 0
		transcoding := 0
		for _, s := range sessions {
			if s.NowPlayingItem != nil {
				active++
				if s.TranscodingInfo != nil {
					transcoding++
				}
			}
		}

		fmt.Fprintf(w, `<div class="service-card"><div><div class="service-name">%s v%s</div>`, info.ServerName, info.Version)
		fmt.Fprintf(w, `<div class="service-meta">%d watching`, active)
		if transcoding > 0 {
			fmt.Fprintf(w, ` (%d transcoding)`, transcoding)
		}
		fmt.Fprintf(w, `</div></div></div>`)
		fmt.Fprintf(w, `<div class="service-card"><div><div class="service-meta">%d movies · %d episodes · %d songs</div></div></div>`,
			counts.MovieCount, counts.EpisodeCount, counts.SongCount)
	}
}

// FrigateSummaryFragment returns the Frigate summary widget.
func FrigateSummaryFragment(client FrigateClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		version, versionErr := client.GetVersion(ctx)
		if versionErr != nil {
			fragmentError(w, versionErr, "Frigate unreachable")
			return
		}

		stats, _ := client.GetStats(ctx)

		w.Header().Set("Content-Type", "text/html")

		if versionErr != nil {
			fmt.Fprint(w, `<p class="muted">Frigate unreachable</p>`)
			return
		}

		totalCameras := len(stats.Cameras)
		activeCameras := 0
		for _, cam := range stats.Cameras {
			if cam.DetectionEnabled {
				activeCameras++
			}
		}

		uptime := stats.Service.Uptime
		uptimeStr := formatUptime(int64(uptime))

		fmt.Fprintf(w, `<div class="service-card"><div><div class="service-name">Frigate v%s</div>`, version)
		fmt.Fprintf(w, `<div class="service-meta">%d/%d cameras active · Up %s</div></div></div>`,
			activeCameras, totalCameras, uptimeStr)
	}
}

// SecuritySummaryFragment returns the security summary badges for htmx refresh.
func SecuritySummaryFragment(tmpl *template.Template, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		summary, _ := s.GetFindingsSummary(r.Context())

		data := struct {
			Summary map[string]int
		}{Summary: summary}

		w.Header().Set("Content-Type", "text/html")
		tmpl.ExecuteTemplate(w, "fragment-security-summary", data)
	}
}
