# Security Policy

## Reporting a vulnerability

Email **daniyal.ibrahim10@gmail.com** with a description and reproduction.
Do not open a public issue for security reports. Expect an acknowledgment
within 72 hours.

## Threat model summary (see docs/security.md for the full text)

- Join codes are 10-char Crockford base32 (~50 bits), bcrypt-hashed at rest,
  valid 15 minutes, single-use, consumed atomically. Unknown/expired/
  consumed codes share one response shape (no oracle).
- Worker tokens are 256-bit opaque, shown once, SHA-256 at rest, revocable.
  Browser-worker tokens expire after 24 h. A stolen worker token steals
  compute *credit*, never money — pools pay the wallet directly.
- The public `/join` page requires a valid code, explicit consent, defaults
  to a 30% duty cycle (hard cap 50%), pauses on hidden tabs, and always
  shows whose swarm it feeds. **Only mine on devices you own or have
  explicit permission to use.**
- The app never asks for, stores, or transmits private keys or seed
  phrases. There is no balance and no withdrawal function; payouts are
  on-chain and explorer-linked.
- User-supplied pool URLs are scheme-allowlisted and DNS-checked against
  loopback/RFC1918 targets (SSRF defense).

## Miner binaries and antivirus false positives

Third-party mining binaries (xmrig, lolMiner, kawpowminer, …) trip antivirus
heuristics — cryptojackers abuse the same tools, so scanners flag the
category, not necessarily the file. money-miner **never bundles** those
binaries in its releases or images. You supply them yourself at setup:

1. Download only from the vendor's **official GitHub releases** (linked in
   `docs/coins.md` per coin).
2. Verify the vendor's published SHA-256 before running anything.
3. Record the checksum in the currency's `adapter_config.sha256` map for
   your platform — the adapter engine verifies it on every start and
   refuses a mismatch.

## Our own release artifacts

Every GitHub release ships `sha256sums.txt` covering all six worker
binaries and the server builds. Verify before running:

```bash
sha256sum -c sha256sums.txt --ignore-missing
```

Builds use `-trimpath` and `CGO_ENABLED=0` static binaries.

## Responsible use

Running this software to mine on machines you do not own or lack
permission to use is unauthorized use of someone else's hardware. Don't.
The browser-mining page exists so owners can *voluntarily* point a phone at
their own swarm with informed consent.
