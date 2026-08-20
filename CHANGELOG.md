# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-08-20

First public release — the honest MVP.

### Added

- Master server (Go, one module): versioned `/v1` REST API per
  `money-miner-api/openapi.yaml`, Keycloak RS256 auth (JWKS), per-user
  tenancy, SSE `/v1/events`, worker WebSocket control channel
  `/v1/swarm/ws`.
- Swarm enrollment: one-time join codes (bcrypt, 15-min expiry, single-use,
  atomic consume, oracle-free errors); 256-bit worker tokens (SHA-256 at
  rest, revocable); browser tokens 24 h.
- Engine layer: adapter engine supervising external open-source miners
  (xmrig + lolMiner stats parsers; sha256-pin verification; never
  auto-downloads unpinned binaries); SIMULATED engine env-gated
  (`ALLOW_SIMULATED=true`), API-flagged and UI-badged.
- 20-coin catalog seeded via Flyway (V2) with explorer templates, pool
  seeds and adapter configs; XCH catalogued but deferred (PoST plotting UX
  out of scope).
- Miners: create wizard, start/stop, live allocation, full run history,
  raw metrics (48 h) + hourly rollups (forever).
- Payout observation: pool API poller records payments with txid +
  explorer link; unverifiable rows are flagged `unverified`; no custody,
  no withdrawals, no balances.
- Worker binary cross-compiled for linux/windows/darwin × amd64/arm64
  (CGO_ENABLED=0) with enroll + run commands and exponential-backoff
  reconnect.
- Frontend (React+TS+Vite, pure CSS tokens, no frameworks/CDNs): Dashboard
  with canvas orb, Miners (wizard, detail, history charts), Settings
  (General/Swarm/Wallets/Exchanges/Currencies), public mobile-first `/join`
  page with consent screen and 30%/50% throttle. Complete English + German
  i18n. Dark default theme with light override.
- Docs site (MkDocs Material), CNCF-style community files, CI/release/docs
  GitHub workflows, GHCR images, `sha256sums.txt` release manifest.

### Known limitations (honest list)

- **Native kHeavyHash engine is a verified stub**: kaspad's pow package
  (ISC license) keeps its matrix constructor unexported, so a correct
  share-validated integration is v0.2 scope. KAS mines via the lolMiner
  adapter; the browser/WASM worker reports itself unavailable rather than
  pretending to hash.
- GPU mining is adapter-only (there is no viable pure-Go GPU compute
  stack); GPU adapter configs beyond ERG/RVN/XEL/VTC are tier-3 data.
- Payout detection parses pools exposing a payments array (2Miners-style);
  cumulative-only APIs (SupportXMR-style) are skipped, not faked.
- Adapter allocation changes restart the miner process (xmrig's HTTP API
  is read-only by default) — a seconds-long pool reconnect.

## [Unreleased]

### Planned (v0.2)

- Native pure-Go kHeavyHash engine + browser/WASM mining via master-relay.
- GPU adapters for ERG/RVN/XEL/VTC end-to-end.
- Per-pool payout parsers for the remaining seeded pools.
