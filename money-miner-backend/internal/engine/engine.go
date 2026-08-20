// Package engine defines the mining-engine contract (dossier 01) shared by
// the master server and the swarm worker. One Engine implementation = one
// mining backend (native Go hasher, external-miner adapter, or the env-gated
// SIMULATED demo engine).
//
// Governing constraint: honesty. A running miner produces real hashes
// against a real pool endpoint, or is explicitly labeled SIMULATED in API
// and UI. No silent fakes.
package engine

import "context"

// Engine is one mining backend (dossier 01 — engine interface).
type Engine interface {
	Name() string
	// Prepare validates config + ensures the engine can run on this node
	// (binary present, VRAM sufficient, verthash.dat present, ...).
	Prepare(ctx context.Context, cfg EngineConfig) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error // graceful: unsubmits, kills child
	SetAllocation(a Allocation) error // live CPU%/thread + GPU intensity change
	Stats() EngineStats               // hashrate, shares acc/rej, uptime
}

// GPUAlloc describes a GPU assignment.
type GPUAlloc struct {
	Enabled   bool `json:"enabled"`
	Intensity int  `json:"intensity"` // 0-100
}

// Allocation is a live resource split.
type Allocation struct {
	CPUPct  int `json:"cpu_pct"`
	GPUPct  int `json:"gpu_pct"`
	Threads int `json:"threads"`
}

// EngineConfig is everything an engine needs to point at a pool.
type EngineConfig struct {
	Currency   string `json:"currency"`   // "XMR"
	Algorithm  string `json:"algorithm"`  // "randomx"
	PoolURL    string `json:"pool_url"`   // stratum+tcp://pool.supportxmr.com:443
	Wallet     string `json:"wallet"`     // payout address (pool username)
	WorkerName string `json:"worker_name"` // minerID.workerID pool-side accounting
	Threads    int    `json:"threads"`
	GPU        GPUAlloc         `json:"gpu"`
	Adapter    *AdapterConfig   `json:"adapter,omitempty"`
	Extra      map[string]string `json:"extra,omitempty"`
}

// AdapterConfig mirrors currencies.adapter_config (dossier 01). Binaries
// are fetched at setup from the vendor's official release with pinned
// SHA-256, or user-supplied by path — never bundled in our releases.
type AdapterConfig struct {
	Binary       string            `json:"binary"`
	Version      string            `json:"version"`
	SHA256       map[string]string `json:"sha256"` // "linux/amd64" -> hex
	ArgsTemplate []string          `json:"args_template"`
	APIKind      string            `json:"api_kind"` // xmrig|lolminer|... (parsers implemented for xmrig+lolminer)
	APIPort      int               `json:"api_port"` // 0 = auto-pick free loopback port
	ConfigKind   string            `json:"config_kind,omitempty"`
	Note         string            `json:"note,omitempty"`
}

// EngineStats is the live telemetry snapshot. StatsAvailable=false means
// "the adapter cannot read stats from this binary yet" — never fake numbers.
type EngineStats struct {
	Hashrate       float64 `json:"hashrate"` // H/s normalized
	SharesAccepted int64   `json:"shares_accepted"`
	SharesRejected int64   `json:"shares_rejected"`
	UptimeSeconds  int64   `json:"uptime_s"`
	Running        bool    `json:"running"`
	StatsAvailable bool    `json:"stats_available"`
	Detail         string  `json:"detail,omitempty"`
}
