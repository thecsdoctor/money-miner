//go:build js && wasm

// money-miner browser hasher (GOOS=js GOARCH=wasm) — the /join page's
// mining module. It wraps the native engine.
//
// v0.1.0 HONEST STATE: the native kHeavyHash engine is a verified stub
// (internal/engine/native), so this module reports available=false with the
// full reason instead of pretending to hash. The /join page renders that
// state: consent UI, throttle controls and enrollment are real; hashing
// starts shipping when the native engine lands (v0.2, docs/roadmap.md).
package main

import (
	"syscall/js"

	"github.com/thecsdoctor/money-miner/money-miner-backend/internal/engine/native"
)

func main() {
	js.Global().Set("mmNative", js.ValueOf(map[string]any{
		// available: whether this build can actually hash.
		"available": false,
		// reason: the honest explanation shown on the consent screen.
		"reason": native.Reason(),
		// duty cycle limits per dossier 05/07 (informational for the page).
		"defaultThrottlePct": 30,
		"maxThrottlePct":     50,
	}))
	// start(opts) returns an error string ("" on success). Always the
	// not-verified error in v0.1.0 — the engine refuses before any hashing.
	js.Global().Set("mmStart", js.FuncOf(func(this js.Value, args []js.Value) any {
		return native.Reason()
	}))
	js.Global().Set("mmStop", js.FuncOf(func(this js.Value, args []js.Value) any { return nil }))
	js.Global().Set("mmStats", js.FuncOf(func(this js.Value, args []js.Value) any {
		return map[string]any{"hashrate": 0, "running": false, "stats_available": false}
	}))
	select {} // keep the wasm runtime alive
}
