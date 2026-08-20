# Quickstart

Five minutes from zero to a real share at a real pool.

## 1. Log in

Open the app URL, sign in via your realm account. You land on the dashboard:
empty orb, 20-coin catalog loaded.

## 2. Attach a payout wallet

Settings → Wallets → Add wallet. Pick a currency, paste the **address**
(a label helps). The server validates the format per coin family and rejects
typos. We will never ask for your seed phrase — the pool pays this address
directly.

## 3. Create a miner

Miners → New miner. The wizard walks currency → wallet/pool → allocation →
review. XMR on CPU via the xmrig adapter is the flagship Tier-1 path.
Deferred coins (XCH) are visible but not selectable; GPU coins show their
VRAM gate.

## 4. Add a worker

Settings → Swarm → Generate join code. You get a 15-minute, single-use code
and a one-liner. On any machine (including the server's own box):

```bash
# download the release binary for your OS/arch, verify sha256sums.txt
money-miner-worker enroll https://your-app.example XXXX-XXXX-XX
# install xmrig into ~/.config/money-miner/adapters/ (official release!)
money-miner-worker run
```

The worker connects outbound-only (WSS) — no inbound ports, no firewall
changes, CGNAT-proof.

## 5. Start mining

Dashboard or Miners → Start. The miner queues until a worker connects, the
worker renders the adapter config, starts xmrig, and reports metrics every
15 s. The orb lights up.

**Verify outside our UI (anti-fraud habit):** open your pool's dashboard,
enter your wallet address, and watch your worker appear. Real shares at a
real pool — that's the whole point.

## 6. Watch payouts arrive

Pools pay above their threshold, straight to your wallet. money-miner's
payout poller notices payments via pool APIs and lists them with the
on-chain explorer link. There is no balance and no withdrawal button —
check the chain, not us.

## Phones

Open `/join` on a phone (link in Settings → Swarm), enter a fresh join code,
read the consent screen. In v0.1.0 the browser hasher reports itself
**unavailable** — browser mining ships with the native kHeavyHash engine in
v0.2 (see [Roadmap](roadmap.md)). We tell you that on the page instead of
faking hashrate.
