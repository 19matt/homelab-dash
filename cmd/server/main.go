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

	// Create SSE hub
	hub := web.NewHub()

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

	// Optional: Proxmox integration (multi-node)
	var vmCollectors []*proxmox.VMCollector
	if cfg.Integrations.Proxmox.Enabled {
		pveClient := proxmox.NewClient(cfg.Integrations.Proxmox)

		for _, node := range cfg.Integrations.Proxmox.Nodes {
			sched.AddCollectorWithInterval(proxmox.NewNodeCollector(pveClient, node), 30*time.Second)

			vmCollector := proxmox.NewVMCollector(pveClient, node)
			vmCollectors = append(vmCollectors, vmCollector)
			sched.AddCollectorWithInterval(vmCollector, 30*time.Second)

			sched.AddCheckerWithInterval(proxmox.NewVMChecker(pveClient, node), cfg.Interval)
		}

		log.Printf("proxmox integration enabled: nodes=%v", cfg.Integrations.Proxmox.Nodes)
	}

	// Wire broadcast to SSE hub
	sched.SetBroadcastFunc(func(target, check, status string, latencyMs int64) {
		hub.Broadcast(web.SSEEvent{
			Target:    target,
			Check:     check,
			Status:    status,
			LatencyMs: latencyMs,
		})
	})

	// Create web server with all routes
	srv, err := web.NewServer(cfg.Server, s, hub, vmCollectors)
	if err != nil {
		log.Fatalf("failed to create web server: %v", err)
	}

	// Set up graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start scheduler
	go sched.Run(ctx)

	// Start HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: srv.Handler,
	}

	go func() {
		log.Printf("starting server on %s (interval: %s)", addr, cfg.Interval)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down server...")

	shutdownCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}

	log.Println("server stopped")
}
