package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPingChecker_Success(t *testing.T) {
	// Use localhost which should be reachable
	c := NewPingChecker("test", "127.0.0.1", 22) // SSH port, usually open

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := c.Check(ctx)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if result.Target != "test" {
		t.Errorf("expected target 'test', got '%s'", result.Target)
	}

	if result.Check != "ping" {
		t.Errorf("expected check 'ping', got '%s'", result.Check)
	}

	if result.Latency <= 0 {
		t.Error("expected positive latency")
	}
}

func TestPingChecker_Failure(t *testing.T) {
	c := NewPingChecker("test", "192.0.2.1", 9999) // TEST-NET-1, unreachable

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := c.Check(ctx)
	if err != nil {
		t.Fatalf("Check returned error (should return result): %v", err)
	}

	if result.Status != StatusFail {
		t.Errorf("expected StatusFail, got %s", result.Status)
	}
}

func TestHTTPChecker_200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	c := NewHTTPChecker("test", "127.0.0.1", 0, "http", false)
	// Override the URL to use the test server
	c.url = server.URL

	ctx := context.Background()
	result, err := c.Check(ctx)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if result.Status != StatusPass {
		t.Errorf("expected StatusPass, got %s (message: %s)", result.Status, result.Message)
	}
}

func TestHTTPChecker_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := NewHTTPChecker("test", "127.0.0.1", 0, "http", false)
	c.url = server.URL

	ctx := context.Background()
	result, err := c.Check(ctx)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if result.Status != StatusWarn {
		t.Errorf("expected StatusWarn for 404, got %s", result.Status)
	}
}

func TestHTTPChecker_500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := NewHTTPChecker("test", "127.0.0.1", 0, "http", false)
	c.url = server.URL

	ctx := context.Background()
	result, err := c.Check(ctx)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if result.Status != StatusFail {
		t.Errorf("expected StatusFail for 500, got %s", result.Status)
	}
}

func TestHTTPChecker_Name(t *testing.T) {
	c := NewHTTPChecker("test", "127.0.0.1", 8080, "http", false)
	if c.Name() != "http:8080" {
		t.Errorf("expected 'http:8080', got '%s'", c.Name())
	}
}
