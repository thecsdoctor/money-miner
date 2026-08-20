// Package simulated is the demo/CI engine (dossier 01 + 07 hard rules):
//
//   - selectable only when the server runs with ALLOW_SIMULATED=true;
//   - every API object it touches carries "engine": "simulated";
//   - the UI renders a persistent SIMULATED badge on those miners;
//   - its "shares" are never submitted anywhere and never enter payouts.
//
// It produces SYNTHETIC hashrate so UI plumbing can be exercised without a
// pool. It is honest because it can never pretend to be real.
package simulated

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/engine"
)

// Engine synthesizes hashrate proportional to thread count.
type Engine struct {
	mu      sync.Mutex
	cfg     engine.EngineConfig
	running bool
	started time.Time
	acc     int64
}

// New returns a simulated engine.
func New() *Engine { return &Engine{} }

// Name returns "simulated" — this string lands on API objects and badges.
func (e *Engine) Name() string { return "simulated" }

// Prepare stores the config. Never fails: there is nothing real to check.
func (e *Engine) Prepare(_ context.Context, cfg engine.EngineConfig) error {
	e.mu.Lock()
	e.cfg = cfg
	e.mu.Unlock()
	return nil
}

// Start begins synthesizing stats.
func (e *Engine) Start(context.Context) error {
	e.mu.Lock()
	e.running = true
	e.started = time.Now()
	e.mu.Unlock()
	return nil
}

// Stop halts the simulation.
func (e *Engine) Stop(context.Context) error {
	e.mu.Lock()
	e.running = false
	e.mu.Unlock()
	return nil
}

// SetAllocation adjusts the thread count the synth scales with.
func (e *Engine) SetAllocation(a engine.Allocation) error {
	e.mu.Lock()
	e.cfg.Threads = a.Threads
	e.mu.Unlock()
	return nil
}

// Stats returns the synthetic snapshot. Hashrate is derived from threads
// with jitter so charts look alive; shares tick up slowly. These numbers
// NEVER reach a pool and NEVER enter the payouts table.
func (e *Engine) Stats() engine.EngineStats {
	e.mu.Lock()
	defer e.mu.Unlock()
	st := engine.EngineStats{
		Running:        e.running,
		StatsAvailable: true,
		Detail:         "SIMULATED — synthetic demo numbers, no pool involved",
	}
	if !e.running {
		return st
	}
	threads := e.cfg.Threads
	if threads < 1 {
		threads = 1
	}
	// ~450 H/s per thread with ±10% jitter — plainly labeled synthetic.
	base := float64(threads) * 450
	st.Hashrate = base * (0.9 + 0.2*rand.Float64())
	st.UptimeSeconds = int64(time.Since(e.started).Seconds())
	if rand.Float64() < 0.3 {
		e.acc++
	}
	st.SharesAccepted = e.acc
	return st
}
