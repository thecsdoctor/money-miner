# Architecture

```text
                 ┌────────────────────────────────────────────┐
                 │                TLS edge (nginx)            │
                 │   / → frontend     /api/* → server /v1/*   │
                 └───────┬───────────────────────┬────────────┘
                         │                       │
                  ┌──────┴──────┐         ┌──────┴───────┐
                  │  frontend    │         │   server     │
                  │  React SPA   │──/api──▶│  Go (one     │
                  │  (nginx)     │         │   module)    │
                  └──────────────┘         └──┬───┬───┬───┘
                                             │   │   │
                              postgres/flyway│   │   │keycloak (JWKS)
                                             │   │   │
   workers (outbound-only WSS) ──────────────┘   │   └─ pool APIs (payout observer)
      │                                          │
      └─ engines: adapter → xmrig/lolMiner/… ────┼──▶ mining pools ──▶ your wallet
                                                 │         (pools pay the wallet
        browser workers (v0.2, master-relay) ────┘          directly — no custody)
```

## The engine contract

Every mining backend implements one interface
(`internal/engine`):

```go
type Engine interface {
    Name() string
    Prepare(ctx context.Context, cfg EngineConfig) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    SetAllocation(a Allocation) error
    Stats() EngineStats
}
```

Three implementations:

1. **adapter** — supervises an external open-source miner binary (rendered
   args template, HTTP/JSON stats API, kill on stop). This is how real fleet
   managers (HiveOS, awesome-miner) work: supervise, don't reimplement.
   Keeps `CGO_ENABLED=0` so the 6-target worker matrix cross-compiles
   trivially. GPU mining is adapter-only — there is no viable pure-Go GPU
   compute stack, and we say so instead of pretending.
2. **native** — pure-Go hasher (KAS kHeavyHash). In v0.1.0 this is a
   **verified stub**: kaspad's pow package (ISC license) keeps its matrix
   constructor unexported, so a correct share-validated integration is v0.2
   scope. `Prepare` fails with the full explanation rather than fake hashes.
3. **simulated** — synthetic demo numbers, selectable only with
   `ALLOW_SIMULATED=true`, API-flagged (`engine: "simulated"`), UI-badged,
   never submits shares, never writes payouts.

## Data

Postgres 16, schema owned by **Flyway** (`deploy/migrations/`, one-shot
compose service). The Go process never migrates. Raw metric samples live 48
h; hourly rollups live forever (backend retention loop).

## Tenancy & auth

Keycloak issues RS256 tokens; the server validates against the realm JWKS
and scopes every query by `owner_sub = token.sub`. Roles live in
`realm_access.roles`. The SPA does auth-code + PKCE S256 with tokens in
memory and refresh rotation.

## Realtime

- Users: **SSE** `GET /v1/events` (proxy-friendly, auto-reconnect) —
  `worker_joined | worker_left | metrics_tick | miner_status |
  payout_detected | block_found`. `metrics_tick` (5 s) carries the orb
  payload.
- Workers: **WebSocket** `/v1/swarm/ws` — the only WS in the system.

## Payouts: observation, never custody

Pools pay the configured wallet directly. The payout poller watches pool
APIs, records payments with txid + explorer link, and flags anything it
cannot explorer-verify as `unverified`. There is no withdrawal function.
