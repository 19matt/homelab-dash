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
		results, _ := s.GetLatestCheckResults(r.Context())

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
		summary, _ := s.GetFindingsSummary(r.Context())

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
