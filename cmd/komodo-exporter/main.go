package main

import (
	"context"
	"log"
	"net/http"
	"os"
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


func main() {
	baseURL := mustEnv("KOMODO_HOST") 
	apiKey := mustEnv("KOMODO_API_KEY")
	apiSecret := mustEnv("KOMODO_API_SECRET")
	
	scrapeEvery := 5 * time.Second
	listenAddr := ":9109"

	kc := komodo.NewClient(baseURL, apiKey, apiSecret, komodo.ClientOptions{
		Timeout: 15 * time.Second,
	})

	coll := &collector.Collector{
		Komodo:        kc,
		MaxConcurrent: 8,
		OnlyOk:        true,
	}

	// Custom registry (only exports what you register)
	reg := prometheus.NewRegistry()
	exp := exporter.NewPromExporter(reg)

	// Serve metrics from our registry
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	go func() {
		log.Printf("serving metrics on %s/metrics", listenAddr)
		if err := http.ListenAndServe(listenAddr, mux); err != nil {
			log.Fatal(err)
		}
	}()

	ticker := time.NewTicker(scrapeEvery)
	defer ticker.Stop()

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		stats, err := coll.CollectImportantStats(ctx)
		cancel()

		if err != nil {
			log.Printf("collect error: %v", err)
		} else {
			exp.Update(stats)
		}

		<-ticker.C
	}
}