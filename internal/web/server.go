package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"time"

	"github.com/homelab/homelab-dash/internal/config"
	"github.com/homelab/homelab-dash/internal/integration/frigate"
	"github.com/homelab/homelab-dash/internal/integration/jellyfin"
	"github.com/homelab/homelab-dash/internal/integration/proxmox"
	"github.com/homelab/homelab-dash/internal/scheduler"
	"github.com/homelab/homelab-dash/internal/store"
	"github.com/homelab/homelab-dash/internal/web/handlers"
)

//go:embed static templates
var StaticFS embed.FS

// Server holds the HTTP handler and dependencies.
type Server struct {
	Handler http.Handler
}

// NewServer creates a new web server with all routes registered.
func NewServer(
	cfg config.ServerConfig,
	s *store.Store,
	hub *Hub,
	vmCollectors []*proxmox.VMCollector,
	jellyfinClient *jellyfin.Client,
	frigateClient *frigate.Client,
	fullCfg *config.Config,
	sched *scheduler.Scheduler,
	version, buildTime string,
	startTime time.Time,
	integrationsEnabled []string,
) (*Server, error) {
	// Parse templates
	tmpl, err := template.New("").Funcs(handlers.TemplateFuncMap).ParseFS(StaticFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	// Static file server
	staticFS, err := fs.Sub(StaticFS, "static")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Health (public)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("homelab-dash ok"))
	})

	// SSE
	mux.Handle("GET /events", hub)

	// JSON API
	mux.HandleFunc("GET /api/status", handlers.StatusHandler(s))
	mux.HandleFunc("GET /api/uptime", handlers.UptimeHandler(s))
	mux.HandleFunc("GET /api/uptime/daily", handlers.DailyUptimeHandler(s))
	mux.HandleFunc("GET /api/metrics/latest", handlers.MetricsLatestHandler(s))
	mux.HandleFunc("GET /api/metrics/history", handlers.MetricsHistoryHandler(s))
	mux.HandleFunc("GET /api/metrics/targets", handlers.MetricsTargetsHandler(s))
	if len(vmCollectors) > 0 {
		mux.HandleFunc("GET /api/proxmox/vms", handlers.ProxmoxVMsHandlerMulti(vmCollectors))
	}

	// Security API
	mux.HandleFunc("GET /api/security/findings", handlers.FindingsHandler(s))
	mux.HandleFunc("GET /api/security/summary", handlers.SummaryHandler(s))
	mux.HandleFunc("GET /api/security/report", handlers.ReportHandler(s))
	mux.HandleFunc("GET /api/security/resolved", handlers.ResolvedFindingsHandler(s))

	// Alerts API
	mux.HandleFunc("GET /api/alerts/events", handlers.AlertEventsHandler(s))

	// Admin API
	mux.HandleFunc("GET /admin/status", handlers.AdminStatusHandler(s, version, buildTime, startTime, integrationsEnabled))

	// Audit API
	mux.HandleFunc("GET /api/audit/events", handlers.AuditEventsHandler(s))

	// Settings API
	mux.HandleFunc("POST /api/targets", handlers.AddTargetHandler(fullCfg, s))
	mux.HandleFunc("DELETE /api/targets/{name}", handlers.RemoveTargetHandler(fullCfg, s))
	mux.HandleFunc("POST /api/targets/test", handlers.TestTargetHandler())

	// Page handlers
	mux.HandleFunc("GET /{$}", handlers.OverviewHandler(tmpl, s, vmCollectors, jellyfinClient, frigateClient))
	mux.HandleFunc("GET /services", handlers.ServicesHandler(tmpl, s))
	mux.HandleFunc("GET /proxmox", handlers.ProxmoxHandler(tmpl, s, vmCollectors))
	mux.HandleFunc("GET /metrics", handlers.MetricsHandler(tmpl, s))
	mux.HandleFunc("GET /security", handlers.SecurityHandler(tmpl, s))
	mux.HandleFunc("GET /alerts", handlers.AlertsHandler(tmpl, s))
	mux.HandleFunc("GET /host/{target...}", handlers.HostHandler(tmpl, s))
	mux.HandleFunc("GET /settings", handlers.SettingsHandler(tmpl, fullCfg))
	mux.HandleFunc("GET /audit", handlers.AuditHandler(tmpl, s))

	// Conditional integration pages
	if jellyfinClient != nil {
		mux.HandleFunc("GET /jellyfin", handlers.JellyfinHandler(tmpl, jellyfinClient))
	}
	if frigateClient != nil {
		mux.HandleFunc("GET /frigate", handlers.FrigateHandler(tmpl, frigateClient))
	}

	// Htmx fragment handlers
	mux.HandleFunc("GET /fragments/status-grid", handlers.StatusGridFragment(tmpl, s))
	mux.HandleFunc("GET /fragments/proxmox-summary", handlers.ProxmoxSummaryFragment(tmpl, s, vmCollectors))
	mux.HandleFunc("GET /fragments/vm-table", handlers.VMTableFragment(tmpl, vmCollectors))
	mux.HandleFunc("GET /fragments/security-summary", handlers.SecuritySummaryFragment(tmpl, s))
	mux.HandleFunc("GET /fragments/findings-badge", handlers.FindingsBadgeFragment(s))

	if jellyfinClient != nil {
		mux.HandleFunc("GET /fragments/jellyfin-summary", handlers.JellyfinSummaryFragment(jellyfinClient))
	}
	if frigateClient != nil {
		mux.HandleFunc("GET /fragments/frigate-summary", handlers.FrigateSummaryFragment(frigateClient))
	}

	// Wrap with auth if enabled
	var handler http.Handler = mux
	if cfg.Auth.Enabled {
		handler = BasicAuth(cfg.Auth.Username, cfg.Auth.Password, mux, "/health", "/static/")
	}

	return &Server{Handler: handler}, nil
}
