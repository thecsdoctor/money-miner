// Package native is the pure-Go mining engine slot (dossier 01, decision 2:
// "Pure-Go native engine for KAS (kHeavyHash)").
//
// STATUS IN v0.1.0 — VERIFIED STUB, NOT A MINER.
//
// Verification record (dossier open question #1, checked 2026-08-20):
//   - kaspad (github.com/kaspanet/kaspad) is a Go codebase under the ISC
//     license (permissive; "kaspanet developers" copyright).
//   - Its kHeavyHash lives in domain/consensus/utils/pow: heavyhash.go +
//     xoshiro.go implement the matrix-multiply hash, and the package
//     compiles without cgo.
//   - BUT the package is NOT externally usable as a library: the matrix
//     type and generateMatrix are unexported, and the only exported entry
//     point (pow.State via pow.go) requires kaspad consensus block-header
//     types plus exact consensus serialization to produce correct hashes.
//     Vendoring that machinery (or forking the two files) without being
//     able to validate shares against a live pool would risk submitting
//     invalid work — which the honesty rule forbids shipping as "mining".
//
// Decision (dossier 01 fallback): the Engine interface and wiring are
// complete; Prepare returns ErrNotVerified with the full explanation; KAS
// mines via the lolMiner ADAPTER in v0.1.0, and the browser/WASM worker
// (which needs this engine) ships with the native engine in v0.2 — see
// docs/roadmap.md. The /join page and the wasm module report this state
// honestly instead of pretending to hash.
package native

import (
	"context"
	"errors"

	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/engine"
)

// ErrNotVerified is returned by Prepare: the pure-Go kHeavyHash integration
// is verified-pending (see package doc). Never silently fake hashing.
var ErrNotVerified = errors.New("native kHeavyHash engine unavailable in v0.1.0: " +
	"kaspad's pow package keeps its matrix constructor unexported (verified 2026-08-20, ISC license); " +
	"a correct share-validated integration is v0.2 scope (docs/roadmap.md). " +
	"Use engine=adapter (lolMiner) for KAS meanwhile")

// Reason exposes the human-readable explanation for API/UI display
// (join-info browser_mining.reason, /join page).
func Reason() string { return ErrNotVerified.Error() }

// Engine is the stub. It implements engine.Engine so the registry wiring is
// real; only Prepare always fails with ErrNotVerified.
type Engine struct{}

// New returns the native engine stub.
func New() *Engine { return &Engine{} }

// Name returns "native-go".
func (e *Engine) Name() string { return "native-go" }

// Prepare always fails in v0.1.0 — see package doc.
func (e *Engine) Prepare(context.Context, engine.EngineConfig) error { return ErrNotVerified }

// Start is unreachable (Prepare fails first) and fails safe.
func (e *Engine) Start(context.Context) error { return ErrNotVerified }

// Stop is a no-op.
func (e *Engine) Stop(context.Context) error { return nil }

// SetAllocation is a no-op.
func (e *Engine) SetAllocation(engine.Allocation) error { return nil }

// Stats reports a non-running engine with stats explicitly unavailable.
func (e *Engine) Stats() engine.EngineStats {
	return engine.EngineStats{Running: false, StatsAvailable: false, Detail: ErrNotVerified.Error()}
}
