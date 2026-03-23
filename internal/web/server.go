package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/homelab/homelab-dash/internal/config"
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
	deps *ServerDependencies,
	versionInfo *ServerVersion,
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
	mux.Handle("GET /events", deps.Hub)

	// JSON API
	mux.HandleFunc("GET /api/status", handlers.StatusHandler(deps.Store))
	mux.HandleFunc("GET /api/uptime", handlers.UptimeHandler(deps.Store))
	mux.HandleFunc("GET /api/uptime/daily", handlers.DailyUptimeHandler(deps.Store))
	mux.HandleFunc("GET /api/metrics/latest", handlers.MetricsLatestHandler(deps.Store))
	mux.HandleFunc("GET /api/metrics/history", handlers.MetricsHistoryHandler(deps.Store))
	mux.HandleFunc("GET /api/metrics/targets", handlers.MetricsTargetsHandler(deps.Store))
	if len(deps.VMCollectors) > 0 {
		mux.HandleFunc("GET /api/proxmox/vms", handlers.ProxmoxVMsHandlerMulti(deps.VMCollectors))
	}

	// Security API
	mux.HandleFunc("GET /api/security/findings", handlers.FindingsHandler(deps.Store))
	mux.HandleFunc("GET /api/security/summary", handlers.SummaryHandler(deps.Store))
	mux.HandleFunc("GET /api/security/report", handlers.ReportHandler(deps.Store))
	mux.HandleFunc("GET /api/security/resolved", handlers.ResolvedFindingsHandler(deps.Store))

	// Alerts API
	mux.HandleFunc("GET /api/alerts/events", handlers.AlertEventsHandler(deps.Store))

	// Admin API
	mux.HandleFunc("GET /admin/status", handlers.AdminStatusHandler(deps.Store, versionInfo.Version, versionInfo.BuildTime, versionInfo.StartTime, deps.Integrations))

	// Audit API
	mux.HandleFunc("GET /api/audit/events", handlers.AuditEventsHandler(deps.Store))

	// Settings API
	mux.HandleFunc("POST /api/targets", handlers.AddTargetHandler(deps.ConfigManager, deps.Store))
	mux.HandleFunc("DELETE /api/targets/{name}", handlers.RemoveTargetHandler(deps.ConfigManager, deps.Store))
	mux.HandleFunc("POST /api/targets/test", handlers.TestTargetHandler())

	// Page handlers
	mux.HandleFunc("GET /{$}", handlers.OverviewHandler(tmpl, deps.Store, deps.VMCollectors, deps.JellyfinClient, deps.FrigateClient))
	mux.HandleFunc("GET /services", handlers.ServicesHandler(tmpl, deps.Store))
	mux.HandleFunc("GET /proxmox", handlers.ProxmoxHandler(tmpl, deps.Store, deps.VMCollectors))
	mux.HandleFunc("GET /metrics", handlers.MetricsHandler(tmpl, deps.Store))
	mux.HandleFunc("GET /security", handlers.SecurityHandler(tmpl, deps.Store))
	mux.HandleFunc("GET /alerts", handlers.AlertsHandler(tmpl, deps.Store))
	mux.HandleFunc("GET /host/{target...}", handlers.HostHandler(tmpl, deps.Store))
	mux.HandleFunc("GET /settings", handlers.SettingsHandler(tmpl, deps.ConfigManager.Get()))
	mux.HandleFunc("GET /audit", handlers.AuditHandler(tmpl, deps.Store))

	// Conditional integration pages
	if deps.JellyfinClient != nil {
		mux.HandleFunc("GET /jellyfin", handlers.JellyfinHandler(tmpl, deps.JellyfinClient))
	}
	if deps.FrigateClient != nil {
		mux.HandleFunc("GET /frigate", handlers.FrigateHandler(tmpl, deps.FrigateClient))
	}

	// Htmx fragment handlers
	mux.HandleFunc("GET /fragments/status-grid", handlers.StatusGridFragment(tmpl, deps.Store))
	mux.HandleFunc("GET /fragments/proxmox-summary", handlers.ProxmoxSummaryFragment(tmpl, deps.Store, deps.VMCollectors))
	mux.HandleFunc("GET /fragments/vm-table", handlers.VMTableFragment(tmpl, deps.VMCollectors))
	mux.HandleFunc("GET /fragments/security-summary", handlers.SecuritySummaryFragment(tmpl, deps.Store))
	mux.HandleFunc("GET /fragments/findings-badge", handlers.FindingsBadgeFragment(deps.Store))

	if deps.JellyfinClient != nil {
		mux.HandleFunc("GET /fragments/jellyfin-summary", handlers.JellyfinSummaryFragment(deps.JellyfinClient))
	}
	if deps.FrigateClient != nil {
		mux.HandleFunc("GET /fragments/frigate-summary", handlers.FrigateSummaryFragment(deps.FrigateClient))
	}

	// Wrap with auth if enabled
	var handler http.Handler = mux
	if deps.ConfigManager.Get().Server.Auth.Enabled {
		cfg := deps.ConfigManager.Get()
		handler = BasicAuth(cfg.Server.Auth.Username, cfg.Server.Auth.Password, mux, "/health", "/static/")
	}

	return &Server{Handler: handler}, nil
}
