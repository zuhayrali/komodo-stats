package collector

import (
	"context"
	"fmt"
	"sync"

	"komodo-exporter/internal/komodo"
)

type Collector struct {
	Komodo       *komodo.Client
	MaxConcurrent int // e.g. 5-10
	OnlyOk        bool
}

func (c *Collector) CollectImportantStats(ctx context.Context) ([]komodo.ImportantStats, error) {
	servers, err := c.Komodo.ListServers(ctx)
	if err != nil {
		return nil, err
	}

	maxConc := c.MaxConcurrent
	if maxConc <= 0 {
		maxConc = 8
	}

	type result struct {
		stat komodo.ImportantStats
		err  error
	}

	sem := make(chan struct{}, maxConc)
	outCh := make(chan result, len(servers))

	var wg sync.WaitGroup
	for _, s := range servers {
		s := s // capture

		if c.OnlyOk && s.Info.State != "Ok" {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			st, err := c.Komodo.GetSystemStats(ctx, s.ID)
			if err != nil {
				outCh <- result{err: fmt.Errorf("get stats for %s (%s): %w", s.Name, s.ID, err)}
				return
			}

			outCh <- result{
				stat: komodo.ImportantStats{
					ServerID:            s.ID,
					ServerName:          s.Name,
					CPUPerc:             st.CPUPerc,
					MemFreeGB:           st.MemFreeGB,
					MemUsedGB:           st.MemUsedGB,
					MemTotalGB:          st.MemTotalGB,
					NetworkIngressBytes: st.NetworkIngressBytes,
					NetworkEgressBytes:  st.NetworkEgressBytes,
					RefreshTS:           st.RefreshTS,
				},
			}
		}()
	}

	wg.Wait()
	close(outCh)

	stats := make([]komodo.ImportantStats, 0, len(servers))

	// Choose behavior:
	// - If ANY error should fail the scrape, return the first error.
	// - Or accumulate partial results and log errors elsewhere.
	for r := range outCh {
		if r.err != nil {
			return nil, r.err
		}
		stats = append(stats, r.stat)
	}

	return stats, nil
}
