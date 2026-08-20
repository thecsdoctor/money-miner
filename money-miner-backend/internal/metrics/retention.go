// Package metrics runs the rollup/retention loop (dossier 03): hourly,
// aggregate metric_samples into metric_rollups and delete raw rows older
// than 48 h. The Go side owns the policy so it stays testable; Flyway owns
// the schema.
package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/store"
)

// Retention periodically rolls up and prunes metric samples.
type Retention struct {
	st       *store.Store
	interval time.Duration
}

// NewRetention returns the retention runner.
func NewRetention(st *store.Store, interval time.Duration) *Retention {
	return &Retention{st: st, interval: interval}
}

// Run loops until ctx is cancelled; first pass runs immediately.
func (r *Retention) Run(ctx context.Context) {
	t := time.NewTicker(r.interval)
	defer t.Stop()
	r.once(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.once(ctx)
		}
	}
}

func (r *Retention) once(ctx context.Context) {
	if err := r.st.RollupOnce(ctx); err != nil {
		slog.Warn("metrics retention failed", "err", err)
	}
}
