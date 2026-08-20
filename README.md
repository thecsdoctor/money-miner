# money-miner

[![ci](https://github.com/thecsdoctor/money-miner/actions/workflows/ci.yml/badge.svg)](https://github.com/thecsdoctor/money-miner/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/thecsdoctor/money-miner)](https://github.com/thecsdoctor/money-miner/releases)
[![GHCR server](https://img.shields.io/badge/ghcr.io-money--miner--server-blue)](https://github.com/thecsdoctor/money-miner/pkgs/container/money-miner-server)
[![GHCR frontend](https://img.shields.io/badge/ghcr.io-money--miner--frontend-blue)](https://github.com/thecsdoctor/money-miner/pkgs/container/money-miner-frontend)
[![docs](https://github.com/thecsdoctor/money-miner/actions/workflows/docs.yml/badge.svg)](https://thecsdoctor.github.io/money-miner)
[![license](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/thecsdoctor/money-miner/money-miner-backend)](https://goreportcard.com/report/github.com/thecsdoctor/money-miner/money-miner-backend)

A self-hosted cryptocurrency mining **swarm** platform: a master web app
where you create per-coin miners, attach payout wallets, and watch real
statistics — plus a Go worker binary that turns any laptop/desktop into a
swarm node via a one-time join code, and a public browser page that lets a
phone contribute compute with explicit consent.

> **The one rule: no fake mining.** Real stratum, real pools, payouts
> on-chain to your own wallet with explorer links — or an explicitly labeled
> `SIMULATED` mode. The platform never custodies funds and never touches
> private keys.

> **Honest economics:** home mining is hobby-scale. A desktop CPU earns
> cents per day on RandomX-class coins and electricity may cost more than
> the reward. Pools pay out above thresholds only. Nothing here is an
> investment, and no legitimate tool can promise mining profits. The value
> of this project is learning, self-custody, and infrastructure — not
> income.

## Features

- **20-coin catalog** — exactly the research-verified mineable assets
  (XMR, KAS, RTM, ZEPH, ERG, RVN, XEL, VTC, ETC, FLUX, ZEC, ARRR, BTG, GRIN,
  BEAM, CLORE, OCTA, DASH, DGB, XCH). Non-mineable coins never appear. XCH
  is catalogued but deferred (Proof-of-Space plotting UX is out of v0.1
  scope).
- **Miners per currency** with live CPU/GPU allocation, full run history,
  raw metrics (48 h) + hourly rollups (forever).
- **Swarm**: one-time join codes (bcrypt, 15-min, single-use), workers
  enroll outbound-only (HTTPS + WSS, no inbound ports), phones join via the
  public `/join` page (consent screen, 30% default / 50% cap throttle).
- **Engines**: adapter engine supervising external open-source miners
  (xmrig & co., stats via their HTTP APIs, sha256-pinned binaries) — CPU and
  GPU; pure-Go native engine slot (kHeavyHash, verified stub in v0.1.0 —
  see below); env-gated SIMULATED demo engine.
- **Payout tracking** with block-explorer links. The pool pays your wallet
  directly; there is no balance and no withdrawal function.
- **Web app**: React+TS+Vite, pure CSS design tokens (dark default + light
  override), complete English + German i18n, canvas dashboard orb, no UI
  frameworks, no CDNs.
- **Ops**: Postgres + Flyway migrations, Keycloak SSO, SSE dashboard
  updates, docker compose deployment, GHCR images, 6-target worker release
  matrix with `sha256sums.txt`.

## Honest limitations (v0.1.0)

- The **native kHeavyHash engine is a verified stub**: kaspad's pow package
  (ISC license) keeps its matrix constructor unexported, so a correct
  share-validated integration is v0.2 scope. KAS mines via the lolMiner
  adapter meanwhile, and the browser/WASM worker reports itself unavailable
  rather than pretending to hash.
- GPU mining rides external miners (no viable pure-Go GPU stack exists);
  GPU adapters beyond the seeded configs are tier-3 data.
- Payout detection parses pools exposing a payments array (2Miners-style);
  cumulative-only pool APIs are skipped, not faked.

See [CHANGELOG](CHANGELOG.md) and [docs/roadmap](docs/roadmap.md).

## Quickstart

```bash
git clone https://github.com/thecsdoctor/money-miner.git
cd money-miner/deploy && cp .env.example .env   # fill values
docker compose up -d --build
```

Then: log in → Settings → Wallets (attach an address) → Miners → New miner
→ Settings → Swarm (join code) → on any machine:

```bash
money-miner-worker enroll https://your-app.example XXXX-XXXX-XX
money-miner-worker run
```

Full guide: **[thecsdoctor.github.io/money-miner](https://thecsdoctor.github.io/money-miner)**

## Repository layout

```
money-miner-api/        OpenAPI 3 contract of record + swagger-editor
money-miner-backend/    one Go module: cmd/server, cmd/worker, cmd/browserhash (wasm)
money-miner-frontend/   React+TS+Vite SPA, pure CSS, i18n EN/DE
deploy/                 docker-compose.yml + Flyway migrations
docs/ + mkdocs.yml      MkDocs Material site (GitHub Pages)
.github/                ci / release / docs workflows, templates
```

## Responsible use

Mine only on devices you own or have explicit permission to use. The
browser-mining page exists so owners can voluntarily point a phone at their
own swarm with informed consent. Third-party miner binaries are never
bundled — fetch them from vendors' official releases and verify checksums
(see [SECURITY.md](SECURITY.md)).
