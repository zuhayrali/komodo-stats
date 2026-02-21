package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"komodo-exporter/internal/collector"
	"komodo-exporter/internal/exporter"
	"komodo-exporter/internal/komodo"
)

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var %s", key)
	}
	return v
}

func envBool(key string, def bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return def
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Fatalf("invalid %s value %q: %v", key, value, err)
	}

	return parsed
}

func envString(key, def string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return def
	}
	return value
}

type scraper struct {
	collector *collector.Collector
	exporter  *exporter.PromExporter
	metrics   *exporter.ScrapeMetrics
	mu        sync.Mutex
}

func (s *scraper) run(ctx context.Context) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    start := time.Now()
    stats, err := s.collector.CollectImportantStats(ctx)
    s.exporter.Reset()
    s.exporter.Update(stats)
    s.metrics.Observe(err, time.Since(start))
    return err
}

func main() {
	baseURL := mustEnv("KOMODO_HOST") 
	apiKey := mustEnv("KOMODO_API_KEY")
	apiSecret := mustEnv("KOMODO_API_SECRET")
	
	listenAddr := envString("KOMODO_LISTEN_ADDR", ":9109")
	insecureSkipVerify := envBool("KOMODO_INSECURE_SKIP_VERIFY", false)

	kc := komodo.NewClient(baseURL, apiKey, apiSecret, komodo.ClientOptions{
		InsecureSkipVerify: insecureSkipVerify,
	})

	coll := &collector.Collector{
		Komodo: kc,
	}

	// Custom registry (only exports what you register)
	reg := prometheus.NewRegistry()
	exp := exporter.NewPromExporter(reg)
	metrics := exporter.NewScrapeMetrics(reg)
	worker := &scraper{
		collector: coll,
		exporter:  exp,
		metrics:   metrics,
	}

	// Serve metrics from our registry
	mux := http.NewServeMux()
	baseMetricsHandler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	metricsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		if err := worker.run(ctx); err != nil {
			log.Printf("collect error: %v", err)
		}
		baseMetricsHandler.ServeHTTP(w, r)
	})

	mux.Handle("/metrics", metricsHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	go func() {
		log.Printf("serving metrics on http://localhost%s/metrics", listenAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server error: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("http server shutdown error: %v", err)
	}
}
