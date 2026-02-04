package exporter

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"komodo-exporter/internal/komodo"
)

type PromExporter struct {
	cpuPerc  *prometheus.GaugeVec
	memFree  *prometheus.GaugeVec
	memUsed  *prometheus.GaugeVec
	memTotal *prometheus.GaugeVec
	netIn    *prometheus.GaugeVec
	netOut   *prometheus.GaugeVec
}

func NewPromExporter(reg prometheus.Registerer) *PromExporter {
	e := &PromExporter{
		cpuPerc: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "komodo_cpu_perc", Help: "CPU percent"},
			[]string{"server_id", "server_name"},
		),
		memFree: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "komodo_mem_free_gb", Help: "Free memory (GB)"},
			[]string{"server_id", "server_name"},
		),
		memUsed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "komodo_mem_used_gb", Help: "Used memory (GB)"},
			[]string{"server_id", "server_name"},
		),
		memTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "komodo_mem_total_gb", Help: "Total memory (GB)"},
			[]string{"server_id", "server_name"},
		),
		netIn: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "komodo_network_ingress_bytes", Help: "Network ingress bytes"},
			[]string{"server_id", "server_name"},
		),
		netOut: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "komodo_network_egress_bytes", Help: "Network egress bytes"},
			[]string{"server_id", "server_name"},
		),
	}

	reg.MustRegister(e.cpuPerc, e.memFree, e.memUsed, e.memTotal, e.netIn, e.netOut)
	return e
}

func (e *PromExporter) Update(stats []komodo.ImportantStats) {
	for _, s := range stats {
		labels := prometheus.Labels{"server_id": s.ServerID, "server_name": s.ServerName}
		e.cpuPerc.With(labels).Set(s.CPUPerc)
		e.memFree.With(labels).Set(s.MemFreeGB)
		e.memUsed.With(labels).Set(s.MemUsedGB)
		e.memTotal.With(labels).Set(s.MemTotalGB)
		e.netIn.With(labels).Set(s.NetworkIngressBytes)
		e.netOut.With(labels).Set(s.NetworkEgressBytes)
	}
}

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
