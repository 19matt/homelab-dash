package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
	"github.com/homelab/homelab-dash/internal/config"
	"github.com/homelab/homelab-dash/internal/integration/proxmox"
	"github.com/homelab/homelab-dash/internal/scheduler"
	"github.com/homelab/homelab-dash/internal/store"
	"github.com/homelab/homelab-dash/internal/web"
	"github.com/homelab/homelab-dash/internal/web/handlers"
)

func main() {
	configPath := flag.String("config", "config/homelab.yaml", "path to config file")
	dbPath := flag.String("db", "homelab.db", "path to SQLite database")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	s, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer s.Close()

	// Register service checkers from config
	sched := scheduler.New(s, cfg.Interval)
	for _, t := range cfg.Targets {
		for _, check := range t.Checks {
			switch check {
			case "ping":
				port := 80
				if len(t.Endpoint) > 0 {
					port = t.Endpoint[0].Port
				}
				sched.AddChecker(checker.NewPingChecker(t.Name, t.Host, port))
			case "http":
				for _, ep := range t.Endpoint {
					sched.AddChecker(checker.NewHTTPChecker(t.Name, t.Host, ep.Port, ep.Protocol))
				}
			}
		}
	}

	// Optional: Proxmox integration
	var vmCollector *proxmox.VMCollector
	if cfg.Integrations.Proxmox.Enabled {
		pveClient := proxmox.NewClient(cfg.Integrations.Proxmox)
		node := cfg.Integrations.Proxmox.Node

		sched.AddCollectorWithInterval(proxmox.NewNodeCollector(pveClient, node), 30*time.Second)

		vmCollector = proxmox.NewVMCollector(pveClient, node)
		sched.AddCollectorWithInterval(vmCollector, 30*time.Second)

		sched.AddCheckerWithInterval(proxmox.NewVMChecker(pveClient, node), cfg.Interval)

		log.Printf("proxmox integration enabled: node=%s", node)
	}

	// Build HTTP mux
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "homelab-dash ok")
	})

	// Status and uptime
	mux.HandleFunc("GET /api/status", handlers.StatusHandler(s))
	mux.HandleFunc("GET /api/uptime", handlers.UptimeHandler(s))

	// Metrics
	mux.HandleFunc("GET /api/metrics/latest", handlers.MetricsLatestHandler(s))
	mux.HandleFunc("GET /api/metrics/history", handlers.MetricsHistoryHandler(s))
	mux.HandleFunc("GET /api/metrics/targets", handlers.MetricsTargetsHandler(s))

	// Proxmox VMs (only if enabled)
	if vmCollector != nil {
		mux.HandleFunc("GET /api/proxmox/vms", handlers.ProxmoxVMsHandler(vmCollector))
	}

	// Wrap with auth if enabled (health stays public)
	var handler http.Handler = mux
	if cfg.Server.Auth.Enabled {
		handler = web.BasicAuth(cfg.Server.Auth.Username, cfg.Server.Auth.Password, mux, "/health")
	}

	// Set up graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start scheduler
	go sched.Run(ctx)

	// Start HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		log.Printf("starting server on %s (interval: %s)", addr, cfg.Interval)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down server...")

	shutdownCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}

	log.Println("server stopped")
}
