# Installation

## Requirements

- Docker with compose.
- Postgres reachable by the server (a database + role; this repo's reference
  deployment uses a shared postgres container on an `infra` docker network).
- A Keycloak realm for SSO (public auth-code + PKCE client, bearer-only
  backend client), or adjust `MM_OIDC_*` to any OIDC provider exposing JWKS.

## Reference deployment (docker compose)

```bash
git clone https://github.com/thecsdoctor/money-miner.git
cd money-miner/deploy
cp .env.example .env    # fill values; chmod 600
docker compose up -d --build
```

Services:

| Service | What | Publish |
| --- | --- | --- |
| `migrate` | one-shot Flyway (schema owner) | — |
| `server` | master API + swarm hub | `127.0.0.1:8080` |
| `frontend` | web app (nginx) | `127.0.0.1:3300` |

Publishes are loopback-only by design: put your TLS edge (nginx/caddy/…)
in front, proxy `/` to the frontend and `/api/` to the server with the
`/api/ → /v1/` rewrite. The worker control channel is
`GET /api/swarm/ws` (WebSocket upgrade) and the user event stream is
`GET /api/events` (SSE — disable proxy buffering).

### Environment (server)

See `money-miner-backend/.env.example`. Essentials:

- `MM_DATABASE_URL` — postgres DSN
- `MM_PUBLIC_URL` — external app URL
- `MM_OIDC_ISSUER` / `MM_OIDC_JWKS_URL` — realm issuer + JWKS endpoint
- `ALLOW_SIMULATED` — enable the demo engine (badged everywhere; default off)
- `SEED_TEST_DATA` — seed a demo wallet+miner for the test user (dev only)

### TLS edge snippet (nginx)

```nginx
location / { proxy_pass http://127.0.0.1:3300; ... }
location = /api/swarm/ws {           # WebSocket control channel
    proxy_pass http://127.0.0.1:8080/v1/swarm/ws;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 3600;
}
location = /api/events {             # SSE — no buffering
    proxy_pass http://127.0.0.1:8080/v1/events;
    proxy_buffering off;
    proxy_read_timeout 3600;
}
location /api/ { proxy_pass http://127.0.0.1:8080/v1/; ... }
```

## Miner binaries (supply chain)

money-miner never bundles third-party miner binaries. Install xmrig (and
optionally lolMiner & co.) on each worker from the vendor's official GitHub
release, verify the SHA-256, and place the binary in the worker's adapters
directory (`~/.config/money-miner/adapters/` by default) or on `PATH`.
Pin the checksum in the currency's `adapter_config.sha256` and the adapter
verifies it at every start.
