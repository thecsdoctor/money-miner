# Security & Honesty

## Threat model

Assets: join codes, worker tokens, wallet addresses (integrity, not
secrecy), hashrate attribution (a stolen worker token steals compute
*credit*, not money — pools pay the wallet, never the worker), user session
JWTs, the host box. Actors: internet randos at the public edge, a malicious
"worker" owner, someone embedding the `/join` URL into a third-party site
(cryptojacking-by-proxy), supply-chain compromise of miner binaries.

## Secure-by-default decisions

- **Join codes**: 10-char Crockford base32 (~50 bits, `crypto/rand`),
  bcrypt(10) at rest, 15-min expiry, single-use, atomic consume, revocable,
  oracle-free errors, 10/min/IP redemption limit.
- **Worker tokens**: 256-bit opaque, shown once, SHA-256 at rest,
  revocable; browser tokens expire after 24 h. Least-privilege scope: a
  worker reports only for its own assigned jobs.
- **Wallets**: addresses only. The app never asks for, stores, or transmits
  private keys or seed phrases — the wallet form says exactly that. Per-coin
  format validation on save; invalid means rejected, never stored. Payouts
  are pool→wallet on-chain; there is no withdrawal function.
- **Web baseline**: auth-code + PKCE S256, tokens in memory with refresh
  rotation, parameterized SQL only, no `dangerouslySetInnerHTML`, CSP
  without CDN origins, CORS locked to the app origin, HSTS at the edge.
- **SSRF**: user pool URLs are scheme-allowlisted (`stratum+tcp`,
  `stratum+ssl`, `http(s)`), credential-free, DNS-resolved and rejected when
  they point at loopback/RFC1918. Pools are public; an inward URL is an
  attack.
- **Rate limits**: 300/min/user authenticated; public POSTs 10-30/min/IP;
  SSE 5 concurrent/user.
- **Supply chain**: pinned base images by digest, `govulncheck`-friendly
  minimal deps, third-party GH Actions pinned, external miner binaries never
  bundled and verified against pinned SHA-256 when configured, our own
  releases ship `sha256sums.txt`.

## Honesty rules (from the owner's research)

- No fake hashrates, no off-chain "balances", no withdrawal gates — there
  is no balance at all; payouts are on-chain and explorer-linked.
- The `SIMULATED` engine is env-gated (`ALLOW_SIMULATED=true`), API-flagged
  (`"engine": "simulated"` on every object it touches), carries a persistent
  UI badge, never submits shares, and never writes to the payouts table.
- Unavailable data renders as unavailable (e.g. adapter stats for an
  api_kind without a parser) — never as an invented number.
- Home-mining economics are stated honestly in the README, the UI, and the
  docs: hobby-scale cents-per-day, electricity may cost more, pools pay
  above thresholds, nothing here is an investment, and no legitimate tool
  can promise mining profits.

## Miner binaries and antivirus

Mining binaries trip AV heuristics because cryptojackers abuse the same
tools. money-miner never ships them; fetch them from the vendor's official
GitHub release, verify the vendor SHA-256, and pin it in
`adapter_config.sha256` — the adapter refuses a mismatch on every start.
Verify our own binaries against the release's `sha256sums.txt`.

## Reporting

Security issues: **<daniyal.ibrahim10@gmail.com>** (no public issues for
reports, please). Acknowledgment within 72 h.
