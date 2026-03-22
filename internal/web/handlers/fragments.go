package handlers

import (
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
