package komodo

type ReadRequest struct {
	Type   string      `json:"type"`
	Params interface{} `json:"params"`
}

type ListServersParams struct{}

type ListServersResponseItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Info struct {
		State string `json:"state"`
	} `json:"info"`
}

type GetSystemStatsParams struct {
	Server string `json:"server"`
}

type SystemStats struct {
	CPUPerc             float64 `json:"cpu_perc"`
	MemFreeGB           float64 `json:"mem_free_gb"`
	MemUsedGB           float64 `json:"mem_used_gb"`
	MemTotalGB          float64 `json:"mem_total_gb"`
	NetworkIngressBytes float64 `json:"network_ingress_bytes"`
	NetworkEgressBytes  float64 `json:"network_egress_bytes"`
	RefreshTS           int64   `json:"refresh_ts"`
	PollingRate         string  `json:"polling_rate"`
}

type ImportantStats struct {
	ServerID            string
	ServerName          string
	CPUPerc             float64
	MemFreeGB           float64
	MemUsedGB           float64
	MemTotalGB          float64
	NetworkIngressBytes float64
	NetworkEgressBytes  float64
	RefreshTS           int64
}
