# Contributing to money-miner

Thanks for considering a contribution. The project is a self-hosted
crypto-mining swarm platform with one governing rule: **no fake mining**.
Real stratum, real pools, on-chain payouts to the user's own wallet — or an
explicitly labeled `SIMULATED` mode. Keep it that way.

## Dev setup

```bash
# one-time: deps are Go 1.26+, Node 22+, Docker
make build-wasm            # browser hasher into the frontend
cd deploy && cp .env.example .env  # fill values (see project AGENTS.md)
docker compose up -d --build       # migrate + server + frontend
```

- Backend: `cd money-miner-backend && go run ./cmd/server` (env per
  `.env.example`).
- Frontend: `cd money-miner-frontend && npm install && npm run dev`
  (vite proxies `/api` to `127.0.0.1:8080`).
- API contract: `money-miner-api/openapi.yaml` is the contract of record;
  `make -C money-miner-api editor` serves swagger-editor on
  `127.0.0.1:3301`. If you change the API, change the YAML first.

## Rules for patches

- `go vet`, `go test ./...`, `tsc --noEmit`, and the OpenAPI validation must
  pass (`make test vet lint`; CI runs them).
- Exactly the 20 research-verified mineable coins exist in the catalog
  (deploy/migrations V2). Do not add non-mineable assets. Ever.
- Never fake hashrates, shares, or payouts. Unavailable data renders as
  unavailable. The SIMULATED engine stays env-gated and UI-badged.
- No private keys, seed phrases, or custody features. Wallets are payout
  addresses only. There is no withdrawal function by design.
- No UI frameworks, no CDNs, no webfonts in the frontend (pure CSS tokens).
- Sign-off: add `Signed-off-by: Your Name <you@example.com>` (DCO) to
  commits.
