// Package metrics runs the hourly rollup/retention job (dossier 03 V3):
// aggregate metric_samples → metric_rollups, delete raw rows older than
// 48 h. Backend-side Go by design — testable, not a hidden SQL trigger.
package metrics

import (
	"context"
	"log"
	"time"

	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/store"
)

// RunRollupLoop aggregates + prunes once per interval until ctx ends.
func RunRollupLoop(ctx context.Context, st *store.Store, every time.Duration) {
	if every <= 0 {
		every = time.Hour
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if err := st.RollupOnce(ctx); err != nil {
			log.Printf("metrics: rollup: %v", err)
		}
	}
}
