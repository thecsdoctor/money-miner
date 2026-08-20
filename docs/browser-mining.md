# Browser (phone) mining

The public `/join` page lets a phone contribute compute to your swarm —
with informed, explicit consent as the anti-cryptojacking defense.

## The honest state in v0.1.0

**Browser mining is not available yet.** The browser hasher is a Go→wasm
build of the *native* engine, and the native kHeavyHash engine is a verified
stub (kaspad's pow package keeps its matrix constructor unexported — see
[Architecture](architecture.md)). The `/join` page therefore renders
"browser mining arrives in v0.2" with the full technical reason instead of
pretending to hash. Everything else on the page is real: consent screen,
code validation, throttle controls, enrollment.

## What ships with the native engine (v0.2 design)

- The master holds one stratum connection per browser-mineable miner
  (**master-relay** — browsers can't open raw TCP), translates pool jobs to
  WS messages, and submits candidate nonces upstream after validation
  (target check, dedupe by `(job_id, nonce)`). Shares are real pool shares.
- Pool vardiff is clamped to a floor yielding ~1 share / 30 s / phone.
- Duty-cycle throttling: default 30%, **hard cap 50%**. Auto-pause on
  hidden tab unless the user toggles background mining. Battery status via
  `navigator.getBattery` where available.
- Browser tokens expire after 24 h; re-join needs a fresh code.
- Per-IP concurrency cap (≤3 browser workers/IP), Origin/Referer checks,
  CSP `default-src 'self'`, no third-party embeds, no cookies, no tracking.

## The consent contract (dossier 07)

The page mines in a browser *by design*; the defense is that it must be
unambiguous and consensual:

1. A valid one-time join code is required — no code, no work, ever.
2. An explicit consent screen appears before any hashing starts and names
   the swarm the device contributes to (app name + domain).
3. A giant stop button is always visible.
4. The UI states plainly that phone hashrate is modest.

**Only mine on devices you own or have explicit permission to use.** That's
in the README, the SECURITY policy, and on the page itself.
