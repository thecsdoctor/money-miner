# Swarm & Workers

The swarm turns any machine into a mining node with a one-time join code.
Workers make **outbound-only** connections (HTTPS enroll + WSS control) — no
inbound ports, no firewall changes, no CGNAT problems.

## Join codes

- 10-char Crockford base32 (~50 bits) from `crypto/rand`, shown as
  `XXXX-XXXX-XX`.
- Stored **bcrypt(cost 10)**, plaintext displayed exactly once in the UI.
- 15-minute expiry, single-use, atomically consumed inside the enrollment
  transaction, revocable while unconsumed.
- Unknown / expired / consumed all return the same 404 body (no oracle),
  rate-limited 10/min/IP.

## Enroll & run

```bash
money-miner-worker enroll https://your-app.example XXXX-XXXX-XX [--name label]
money-miner-worker run [--config ~/.config/money-miner/worker.json]
```

Enroll returns `worker_id` + a 256-bit `worker_token` (shown once; the
server stores SHA-256) + the WSS URL, written to the config file (mode 600).
Reconnects use the token until revoked; the code is never reusable.
Browser-worker tokens expire after 24 h.

## Control channel

`wss://…/api/swarm/ws` with `Authorization: Bearer <worker_token>`.
Envelope: `{"type","id?","ts","payload"}`.

server→worker: `job.assign`, `job.cancel`, `allocation.update`, `ping`,
`worker.revoke` (then close).

worker→server: `hello` (caps), `job.ack`, `metrics` (15 s), `log`, `pong`.

Reconnect: exponential backoff 1 s → 60 s, ±20% jitter, forever. The server
marks a worker `offline` after 45 s without traffic (orb node greys out).

## Worker-direct stratum

Native workers connect to the pool **directly** with `wallet.workername` as
the pool username. The master orchestrates (assigns configs, sets
allocation) and observes (metrics reports). The pool pays the wallet
directly — money-miner never sits in the payout path.

The master-relay (a stratum client in `internal/stratum` translating pool
jobs into WS messages) exists for **browser workers only** and is inactive
in v0.1.0 — see [Browser mining](browser-mining.md).

## Revocation

Settings → Swarm → Revoke: the token row is flagged, the live WS is closed,
and engines stop on next contact. Deleting a worker also revokes it.

## Security posture (dossier 07)

- A stolen worker token steals compute *credit*, never money.
- Workers can report metrics/shares **only for their own assigned jobs** —
  enforced server-side per message.
- WS Origin headers (browsers) are checked against the configured app
  origins; native workers send no Origin.
