package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"komodo-exporter/internal/collector"
	"komodo-exporter/internal/komodo"
)

// fakeKomodo spins up a local HTTP server that mimics the Komodo /read endpoint.
// You give it a list of servers, and per-server stat responses (or errors).
type fakeKomodo struct {
	servers    []komodo.ListServersResponseItem
	stats      map[string]komodo.SystemStats // keyed by server ID
	failServer map[string]bool               // server IDs that should return 500
}

func (f *fakeKomodo) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/read" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var req komodo.ReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	switch req.Type {
	case "ListServers":
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(f.servers)

	case "GetSystemStats":
		// Pull the server ID out of params
		paramBytes, _ := json.Marshal(req.Params)
		var p komodo.GetSystemStatsParams
		json.Unmarshal(paramBytes, &p)

		if f.failServer[p.Server] {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "server stats not available"})
			return
		}

		stats, ok := f.stats[p.Server]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)

	default:
		http.Error(w, "unknown type", http.StatusBadRequest)
	}
}

func (f *fakeKomodo) newServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(f.handler))
}

// newTestCollector wires a Collector to the given fake server URL.
func newTestCollector(url string) *collector.Collector {
	kc := komodo.NewClient(url, "test-key", "test-secret", komodo.ClientOptions{})
	return &collector.Collector{Komodo: kc}
}

// --- Tests ---

// TestAllServersHealthy verifies that when all servers respond successfully,
// we get back stats for every server with the correct values.
func TestAllServersHealthy(t *testing.T) {
	fake := &fakeKomodo{
		servers: []komodo.ListServersResponseItem{
			{ID: "aaa", Name: "server-a", Info: struct{ State string `json:"state"` }{State: "Ok"}},
			{ID: "bbb", Name: "server-b", Info: struct{ State string `json:"state"` }{State: "Ok"}},
		},
		stats: map[string]komodo.SystemStats{
			"aaa": {CPUPerc: 10.0, MemUsedGB: 1.0, MemFreeGB: 3.0, MemTotalGB: 4.0},
			"bbb": {CPUPerc: 20.0, MemUsedGB: 2.0, MemFreeGB: 2.0, MemTotalGB: 4.0},
		},
		failServer: map[string]bool{},
	}

	srv := fake.newServer()
	defer srv.Close()

	coll := newTestCollector(srv.URL)
	stats, err := coll.CollectImportantStats(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(stats))
	}

	// Index by server ID for easy lookup
	byID := make(map[string]komodo.ImportantStats)
	for _, s := range stats {
		byID[s.ServerID] = s
	}

	if byID["aaa"].CPUPerc != 10.0 {
		t.Errorf("server-a CPUPerc: expected 10.0, got %f", byID["aaa"].CPUPerc)
	}
	if byID["bbb"].CPUPerc != 20.0 {
		t.Errorf("server-b CPUPerc: expected 20.0, got %f", byID["bbb"].CPUPerc)
	}
}

// TestOneServerDown verifies the core fix: one server returning 500 does NOT
// cause the collector to return an error or drop stats for healthy servers.
func TestOneServerDown(t *testing.T) {
	fake := &fakeKomodo{
		servers: []komodo.ListServersResponseItem{
			{ID: "aaa", Name: "server-a", Info: struct{ State string `json:"state"` }{State: "Ok"}},
			{ID: "bbb", Name: "server-b", Info: struct{ State string `json:"state"` }{State: "Ok"}},
			{ID: "ccc", Name: "server-c", Info: struct{ State string `json:"state"` }{State: "Ok"}},
		},
		stats: map[string]komodo.SystemStats{
			"aaa": {CPUPerc: 10.0},
			"ccc": {CPUPerc: 30.0},
			// "bbb" intentionally missing — will 500
		},
		failServer: map[string]bool{
			"bbb": true,
		},
	}

	srv := fake.newServer()
	defer srv.Close()

	coll := newTestCollector(srv.URL)
	stats, err := coll.CollectImportantStats(context.Background())

	// Should not return an error — partial failure is logged, not fatal
	if err != nil {
		t.Fatalf("expected no error on partial failure, got: %v", err)
	}

	// Should have 2 stats (aaa and ccc), not 3
	if len(stats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(stats))
	}

	// Verify the down server is absent
	for _, s := range stats {
		if s.ServerID == "bbb" {
			t.Error("server-b should not appear in results when it returned 500")
		}
	}

	// Verify the healthy servers are present
	byID := make(map[string]komodo.ImportantStats)
	for _, s := range stats {
		byID[s.ServerID] = s
	}
	if _, ok := byID["aaa"]; !ok {
		t.Error("server-a should be present")
	}
	if _, ok := byID["ccc"]; !ok {
		t.Error("server-c should be present")
	}
}

// TestAllServersDown verifies that when every server fails, we get back
// an empty slice (not an error) — scrape continues, just with no data.
func TestAllServersDown(t *testing.T) {
	fake := &fakeKomodo{
		servers: []komodo.ListServersResponseItem{
			{ID: "aaa", Name: "server-a", Info: struct{ State string `json:"state"` }{State: "Ok"}},
			{ID: "bbb", Name: "server-b", Info: struct{ State string `json:"state"` }{State: "Ok"}},
		},
		stats: map[string]komodo.SystemStats{},
		failServer: map[string]bool{
			"aaa": true,
			"bbb": true,
		},
	}

	srv := fake.newServer()
	defer srv.Close()

	coll := newTestCollector(srv.URL)
	stats, err := coll.CollectImportantStats(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(stats) != 0 {
		t.Fatalf("expected 0 stats when all servers are down, got %d", len(stats))
	}
}

// TestListServersFails verifies that if the initial ListServers call fails
// (e.g. core is unreachable), the collector returns an error immediately.
func TestListServersFails(t *testing.T) {
	// Server that always returns 500 for everything
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"core unavailable"}`))
	}))
	defer srv.Close()

	coll := newTestCollector(srv.URL)
	_, err := coll.CollectImportantStats(context.Background())

	if err == nil {
		t.Fatal("expected error when ListServers fails, got nil")
	}
}

// TestContextCancellation verifies that cancelling the context before the
// request completes results in an error being handled gracefully.
func TestContextCancellation(t *testing.T) {
	fake := &fakeKomodo{
		servers: []komodo.ListServersResponseItem{
			{ID: "aaa", Name: "server-a", Info: struct{ State string `json:"state"` }{State: "Ok"}},
		},
		stats:      map[string]komodo.SystemStats{"aaa": {CPUPerc: 10.0}},
		failServer: map[string]bool{},
	}

	srv := fake.newServer()
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately before any requests

	coll := newTestCollector(srv.URL)
	_, err := coll.CollectImportantStats(ctx)

	// With a cancelled context, ListServers itself should fail,
	// so we expect a non-nil error here
	if err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
}

// TestNoServers verifies that an empty server list returns an empty slice cleanly.
func TestNoServers(t *testing.T) {
	fake := &fakeKomodo{
		servers:    []komodo.ListServersResponseItem{},
		stats:      map[string]komodo.SystemStats{},
		failServer: map[string]bool{},
	}

	srv := fake.newServer()
	defer srv.Close()

	coll := newTestCollector(srv.URL)
	stats, err := coll.CollectImportantStats(context.Background())

	if err != nil {
		t.Fatalf("expected no error for empty server list, got: %v", err)
	}
	if len(stats) != 0 {
		t.Fatalf("expected 0 stats, got %d", len(stats))
	}
}