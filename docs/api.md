# API Reference

The contract of record is
[`money-miner-api/openapi.yaml`](https://github.com/thecsdoctor/money-miner/blob/main/money-miner-api/openapi.yaml)
(OpenAPI 3.0). Base path `/v1` (served as `/api` at the edge with a path
rewrite).

Highlights:

- **Auth**: Keycloak RS256 bearer; tenant = `sub`; roles in
  `realm_access.roles`.
- **Errors**: `{"error": {"code", "message", "details?"}}` everywhere.
- **Pagination**: `?limit=50&offset=0` → `{items, total}`.
- **Public surface** (rate-limited): `POST /v1/swarm/enroll`,
  `GET /v1/public/join-info`, `WS /v1/swarm/ws`, `GET /v1/healthz`.
  Everything else needs a bearer token.
- **Realtime**: SSE `GET /v1/events` (5 concurrent/user).

<redoc spec-url="https://raw.githubusercontent.com/thecsdoctor/money-miner/main/money-miner-api/openapi.yaml"></redoc>
<script src="https://cdn.redoc.ly/redoc/latest/bundles/redoc.standalone.js"></script>

!!! note "Offline reading"
    The redoc tag above needs network access. For a fully local render run
    the bundled swagger-editor instead:
    `make -C money-miner-api editor` → <http://127.0.0.1:3301/?url=/openapi.yaml>.

## Rate limits

| Surface | Limit |
| --- | --- |
| `POST /v1/swarm/enroll` | 10/min/IP |
| `GET /v1/public/join-info` | 30/min/IP |
| authenticated API | 300/min/user |
| SSE | 5 concurrent streams/user |

## Type generation

`make -C money-miner-api types` regenerates TypeScript types
(openapi-typescript → frontend) and Go types (oapi-codegen → backend).
Generated files are gitignored conveniences; the YAML stays the source of
truth.
