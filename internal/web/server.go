package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/homelab/homelab-dash/internal/config"
	"github.com/homelab/homelab-dash/internal/integration/proxmox"
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
func NewServer(cfg config.ServerConfig, s *store.Store, hub *Hub, vmCollectors []*proxmox.VMCollector) (*Server, error) {
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

	// JSON API (existing)
	mux.HandleFunc("GET /api/status", handlers.StatusHandler(s))
	mux.HandleFunc("GET /api/uptime", handlers.UptimeHandler(s))
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

	// Page handlers
	mux.HandleFunc("GET /{$}", handlers.OverviewHandler(tmpl, s, vmCollectors))
	mux.HandleFunc("GET /services", handlers.ServicesHandler(tmpl, s))
	mux.HandleFunc("GET /proxmox", handlers.ProxmoxHandler(tmpl, s, vmCollectors))
	mux.HandleFunc("GET /metrics", handlers.MetricsHandler(tmpl, s))
	mux.HandleFunc("GET /security", handlers.SecurityHandler(tmpl, s))
	mux.HandleFunc("GET /host/{target...}", handlers.HostHandler(tmpl, s))

	// Htmx fragment handlers
	mux.HandleFunc("GET /fragments/status-grid", handlers.StatusGridFragment(tmpl, s))
	mux.HandleFunc("GET /fragments/proxmox-summary", handlers.ProxmoxSummaryFragment(tmpl, s, vmCollectors))
	mux.HandleFunc("GET /fragments/vm-table", handlers.VMTableFragment(tmpl, vmCollectors))
	mux.HandleFunc("GET /fragments/security-summary", handlers.SecuritySummaryFragment(tmpl, s))

	// Wrap with auth if enabled
	var handler http.Handler = mux
	if cfg.Auth.Enabled {
		handler = BasicAuth(cfg.Auth.Username, cfg.Auth.Password, mux, "/health", "/static/")
	}

	return &Server{Handler: handler}, nil
}
