# money-miner

A self-hosted cryptocurrency mining **swarm** platform: a master web app where
you create per-coin miners, attach payout wallets, and watch real statistics —
plus a Go worker binary that turns any laptop/desktop into a swarm node via a
one-time join code, and a public browser page that lets a phone contribute
compute with explicit consent.

**The one rule: no fake mining.** Real stratum, real pools, payouts on-chain
to your own wallet with explorer links — or an explicitly labeled `SIMULATED`
mode. The platform never custodies funds and never touches private keys.

!!! warning "Honest economics"
    Home mining is a hobby-scale activity: a MiniPC CPU earns cents per day on
    RandomX-class coins and electricity may cost more than the reward. Pools
    pay out only above thresholds. Nothing here is an investment, and no
    legitimate tool can promise mining profits. The value of this project is
    learning, self-custody, and infrastructure — not income.

## What it is

- **Master server** (Go): REST `/v1` API, SSE live updates, swarm WebSocket
  control channel, Keycloak SSO, Postgres storage with Flyway migrations.
- **Worker** (one static Go binary, 6 OS/arch targets): enrolls with a
  one-time code, receives jobs over outbound-only WSS, supervises external
  open-source miners (xmrig & co.) per the adapter pattern.
- **Web app** (React+TS+Vite, pure CSS, EN/DE): dashboard with a live swarm
  orb, miner wizard with run history and allocation sliders, settings,
  public `/join` page for phones.
- **Catalog**: exactly the 20 research-verified mineable coins. Non-mineable
  assets are excluded by policy. Chia is catalogued but deferred.

## What it is not

- Not a cloud-mining service, not a pool, not a custodian, not an exchange.
- Not a GPU-compute reimplementation: GPU mining rides external miners via
  the adapter engine (there is no viable pure-Go GPU stack — see
  [Architecture](architecture.md)).
- Not a promise of profit. See the economics note above.

## Links

- [Installation](installation.md) · [Quickstart](quickstart.md)
- [API contract](../money-miner-api/openapi.yaml) (OpenAPI 3, the contract of record)
- Releases: 6 worker binaries + `sha256sums.txt` per tag
- Container images: `ghcr.io/thecsdoctor/money-miner-server`,
  `ghcr.io/thecsdoctor/money-miner-frontend`
