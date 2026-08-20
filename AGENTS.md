# AGENTS.md — money-miner

Self-hosted cryptocurrency mining **swarm** platform (prototype → v0.1.0
open-source release). Monorepo: <https://github.com/thecsdoctor/money-miner>
(public). The git repo root IS this directory.

**The one rule: no fake mining.** Real stratum, real pools, payouts on-chain
to the user's own wallet — or an explicitly labeled SIMULATED mode. The
platform never custodies funds, never stores private keys, and has no
withdrawal function. Keep every change consistent with that rule.

## Layout

| Dir | What |
| --- | --- |
| `money-miner-api/` | `openapi.yaml` (contract of record) + swagger-editor compose + types Makefile |
| `money-miner-backend/` | ONE Go module: `cmd/server`, `cmd/worker`, `cmd/browserhash` (wasm), `internal/{api,auth,config,engine/{adapter,native,simulated},events,metrics,payouts,ratelimit,store,stratum,swarm,wallet}` |
| `money-miner-frontend/` | React+TS+Vite, pure CSS tokens, i18n EN+DE (complete), canvas orb, public `/join` |
| `deploy/` | `docker-compose.yml` (migrate + server + frontend) + `migrations/` (Flyway V1..V4) |
| `docs/` + `mkdocs.yml` | MkDocs Material → GitHub Pages |
| `.github/workflows/` | ci.yml, release.yml (tag-driven), docs.yml |

## Runtime (this box)

Compose project `money-miner` (`deploy/`):

| Service | Container | Loopback | Notes |
| --- | --- | --- | --- |
| migrate | `money-miner-migrate-1` | — | one-shot Flyway, `restart: "no"` BY DESIGN (a one-shot with `unless-stopped` restart-loops — learned the hard way) |
| server | `money-miner-server-1` | 8080 | API + swarm hub; on `infra` net (reaches `db`, `keycloak` by name) |
| frontend | `money-miner-frontend-1` | 3300 | nginx:alpine, env.js rendered by entrypoint.sh |
| swagger-editor | `money-miner-api-swagger-editor-1` | 3301 | `money-miner-api/` compose; spec bind-mounted |

Public URL: <https://money-miner.thecsdoctor.com> (wildcard DNS).
Vhost: `/etc/nginx/conf.d/money-miner.thecsdoctor.com.conf` — created by
`new-vhost.sh ... 3300`, then **manually extended** (re-running new-vhost.sh
refuses; edits survive): `location /api/` → `http://127.0.0.1:8080/v1/`
(path rewrite), `location = /api/swarm/ws` (WebSocket Upgrade headers,
`proxy_read_timeout 3600`), `location = /api/events` (SSE,
`proxy_buffering off`). No `add_header` inside locations (would drop
server-level HSTS).

## Ports

- **8080** backend, **3300** frontend, **3301** swagger-editor — all
  `127.0.0.1`-only publishes. Check `ss -tln` before changing.

## Secrets / env

- `money-miner-backend/.env` (chmod 600, gitignored): `MM_DATABASE_URL`
  (DB `money_miner` — note the UNDERSCORE: `create-database.sh` rejects
  hyphens), `MM_TEST_USER` / `MM_TEST_USER_PASSWORD` / `MM_TEST_USER_SUB`
  (realm test user `test@money-miner.local`).
- `deploy/.env` (chmod 600, gitignored): `MONEY_MINER_DB_PASSWORD`,
  `MM_PUBLIC_URL`, OIDC URLs, `ALLOW_SIMULATED=true`, `SEED_TEST_DATA=true`
  (demo deployment — the seeded "Demo XMR miner" is SIMULATED and badged).
- `.env.example` files carry no values. Never commit real `.env`
  (`.gitignore` first line; verify `git ls-files | grep -c '\.env$'` = 0).

## Infra dependencies

- **Postgres**: container `postgres`, DB `money_miner`, owner role
  `money_miner` (password in the two .env files). Schema owned by Flyway
  ONLY (`deploy/migrations/`); the Go process never migrates.
- **Keycloak**: realm `money-miner` (`auth.thecsdoctor.com`), clients
  `money-miner-app` (public, PKCE S256) + `money-miner-api` (bearer-only).
  Server fetches JWKS internally via `http://keycloak:8080/...` but validates
  `iss` against the public issuer URL.
- **nginx edge**: see Runtime above. Cert: LE via `issue-cert.sh`.
- **Uptime Kuma**: group `money-miner` on <https://status.thecsdoctor.com/> —
  monitors: app URL, `/api/healthz` via edge, container health via `infra`.

## Bring-up / redeploy

```bash
cd ~/projects/money-miner/deploy && docker compose up -d --build
cd ~/projects/money-miner/money-miner-api && docker compose up -d   # swagger editor
```

Verify: `curl -s https://money-miner.thecsdoctor.com/api/healthz` →
`{"db":"ok","status":"ok",...}`; `docker ps` shows server+frontend healthy.

## Build / release

- Root `Makefile`: `build-worker` (6-target CGO=0 matrix → `dist/`),
  `build-server`, `build-wasm` (→ `frontend/public/wasm/`, gitignored),
  `test`, `vet`, `lint`, `release` (+ `sha256sums.txt`).
- Tag `v*` triggers release.yml: 6 worker binaries + 2 server binaries +
  wasm + `sha256sums.txt` on the GitHub Release; GHCR images
  `money-miner-server` / `money-miner-frontend` tagged `{semver}, latest`.
- `docs/**` pushes trigger docs.yml → `mkdocs gh-deploy --force` →
  <https://thecsdoctor.github.io/money-miner> (Pages source: gh-pages branch).

## Gotchas

- **Native kHeavyHash is a VERIFIED STUB** (`internal/engine/native`):
  kaspad's pow package (ISC) keeps its matrix constructor unexported;
  `Prepare` returns `ErrNotVerified`. KAS mines via the lolMiner adapter;
  the `/join` page reports browser mining unavailable (v0.2). Do not fake it.
- **SIMULATED engine**: only when `ALLOW_SIMULATED=true`; objects carry
  `engine=simulated` + UI badge; never submits shares, never writes payouts.
- **Worker enrollment smoke-tested end-to-end** (2026-08-20): enroll → WSS
  through the edge → job.assign → simulated engine → metrics → orb payload.
- Worker allocation changes restart the miner subprocess (xmrig HTTP API is
  read-only by default) — documented, seconds-long pool reconnect.
- V4 migration exists because worker DELETE hit the
  `join_codes.consumed_by` FK (found by smoke test).
- Adapter binaries are NEVER bundled/downloaded unpinned: user supplies
  xmrig/lolMiner in `~/.config/money-miner/adapters/`; `sha256` pins in
  `adapter_config` are verified per start.
- Pool seed URLs in V2 are best-effort; tier-3 coins may need a user-set
  pool override (Settings → Currencies).
- Frontend SSE uses fetch streaming (EventSource can't set Authorization).
- The 20-coin catalog is a hard policy: no non-mineable coins anywhere.

## For AI agents

Design authority: `/home/dany/money-miner-dossier/` (DOSSIER.md + 8
sections). When in doubt, the dossier wins over this file; update this file
when the service changes. Read `~/CLAUDE.md` (box manual) first for
sudo/secret handling.
