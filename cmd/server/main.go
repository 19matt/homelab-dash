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

	"github.com/homelab/homelab-dash/internal/alert"
	"github.com/homelab/homelab-dash/internal/checker"
	"github.com/homelab/homelab-dash/internal/config"
	"github.com/homelab/homelab-dash/internal/integration/frigate"
	"github.com/homelab/homelab-dash/internal/integration/jellyfin"
	"github.com/homelab/homelab-dash/internal/integration/proxmox"
	"github.com/homelab/homelab-dash/internal/scanner"
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

	// Optional: Jellyfin integration
	var jellyfinClient *jellyfin.Client
	if cfg.Integrations.Jellyfin.Enabled && cfg.Integrations.Jellyfin.Host != "" {
		jellyfinClient = jellyfin.NewClient(cfg.Integrations.Jellyfin)
		sched.AddCollectorWithInterval(jellyfin.NewCollector(jellyfinClient), 60*time.Second)
		sched.AddCheckerWithInterval(jellyfin.NewChecker(jellyfinClient), cfg.Interval)
		log.Printf("jellyfin integration enabled: host=%s", cfg.Integrations.Jellyfin.Host)
	}

	// Optional: Frigate integration
	var frigateClient *frigate.Client
	if cfg.Integrations.Frigate.Enabled && cfg.Integrations.Frigate.Host != "" {
		frigateClient = frigate.NewClient(cfg.Integrations.Frigate)
		sched.AddCollectorWithInterval(frigate.NewCollector(frigateClient), 30*time.Second)
		sched.AddCheckerWithInterval(frigate.NewChecker(frigateClient), cfg.Interval)
		log.Printf("frigate integration enabled: host=%s", cfg.Integrations.Frigate.Host)
	}

	// Security scanner
	scanEngine := scanner.NewEngine(s, cfg.Targets)
	scanEngine.AddScanner(&scanner.PortScanner{})
	scanEngine.AddScanner(&scanner.TLSScanner{})
	scanEngine.AddScanner(&scanner.HeaderScanner{})
	sched.AddScanEngine(scanEngine, cfg.ScanInterval)
	log.Printf("security scanner enabled: interval=%s", cfg.ScanInterval)

	// Alert engine
	rules := parseAlertRules(cfg.Alerts)
	if len(rules) > 0 {
		alertEngine := alert.NewEngine(rules)
		alertEval := alert.NewEvaluator(alertEngine, s)
		sched.AddEvaluator(alertEval, cfg.AlertInterval)
		log.Printf("alert engine enabled: %d rules, interval=%s", len(rules), cfg.AlertInterval)
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
	srv, err := web.NewServer(cfg.Server, s, hub, vmCollectors, jellyfinClient, frigateClient)
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

// parseAlertRules converts config AlertConfigs to alert.Rules.
func parseAlertRules(cfgs []config.AlertConfig) []alert.Rule {
	var rules []alert.Rule
	for _, c := range cfgs {
		rule := alert.Rule{
			Name:      c.Name,
			Condition: alert.RuleCondition(c.Condition),
			Target:    c.Target,
			Metric:    c.Metric,
			Threshold: c.Threshold,
			Webhook:   c.Webhook,
			Severity:  c.Severity,
		}

		if rule.Severity == "" {
			rule.Severity = "warn"
		}

		if c.Duration != "" {
			if d, err := time.ParseDuration(c.Duration); err == nil {
				rule.Duration = d
			}
		}

		rules = append(rules, rule)
	}
	return rules
}
