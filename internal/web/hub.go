package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// SSEEvent represents a check result event for SSE broadcasting.
type SSEEvent struct {
	Target    string `json:"target"`
	Check     string `json:"check"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
}

// Hub manages SSE subscribers and broadcasts events.
type Hub struct {
	mu      sync.RWMutex
	clients map[chan SSEEvent]struct{}
}

// NewHub creates a new SSE hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[chan SSEEvent]struct{}),
	}
}

// Subscribe adds a new SSE client and returns its event channel.
func (h *Hub) Subscribe() chan SSEEvent {
	ch := make(chan SSEEvent, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes an SSE client.
func (h *Hub) Unsubscribe(ch chan SSEEvent) {
	h.mu.Lock()
	delete(h.clients, ch)
	close(ch)
	h.mu.Unlock()
}

// Broadcast sends an event to all connected SSE clients.
func (h *Hub) Broadcast(event SSEEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- event:
		default:
			// Drop if client is slow
		}
	}
}

// ServeHTTP handles SSE connections.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.Subscribe()
	defer h.Unsubscribe(ch)

	// Keepalive ticker
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-ch:
			data, err := json.Marshal(event)
			if err != nil {
				log.Printf("sse: marshal error: %v", err)
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}
