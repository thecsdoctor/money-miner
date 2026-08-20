# Roadmap / Milestones

Each step is independently verifiable. "Done when" means verifiable outside
our own UI — the anti-fraud habit.

## Milestone ladder

- **M0 — Scaffold** ✅ (v0.1.0): repo, compose stack, login works, dashboard
  live, 20-coin catalog visible.
- **M1 — One real coin end-to-end**: create an XMR miner, real shares at a
  real pool via the xmrig adapter. *Done when:* the pool's own dashboard
  shows your worker.
- **M2 — Swarm** ✅ mechanism shipped in v0.1.0: join codes, enroll, worker
  binary. *Done when:* the orb shows 2 nodes and the pool shows 2 workers.
- **M3 — Browser worker** ⏳ v0.2 (blocked on the native engine): phone
  opens `/join`, consents, mines. *Done when:* pool-side hashrate rises
  while the phone says "mining".
- **M4 — GPU adapter** ⏳ v0.2: ERG/RVN/XEL/VTC end-to-end with the VRAM
  gate. *Done when:* real shares from a GPU worker.
- **M5 — Polish** ✅ (v0.1.0): EN+DE complete, docs site live, tag cut.

## v0.2 (planned)

- **Native pure-Go kHeavyHash engine.** Verification record: kaspad
  (`github.com/kaspanet/kaspad`, ISC license) implements kHeavyHash in
  `domain/consensus/utils/pow`, but the matrix type/constructor are
  unexported and the exported entry points pull in consensus serialization.
  The integration needs a maintained fork of the two files (heavyhash.go +
  xoshiro.go, ISC-licensed with attribution) plus share validation against a
  live pool before it may claim to mine. The Engine interface and all wiring
  already exist; `Prepare` fails honestly today.
- Browser/WASM mining via master-relay (`internal/stratum` client is
  already in the tree, exercised when the native engine lands).
- GPU adapters ERG/RVN/XEL/VTC end-to-end; per-pool payout parsers beyond
  the 2Miners-style payments array; wallet checksum validation per family.

## Open questions register (verify before claiming publicly)

1. ~~Pure-Go kHeavyHash exists?~~ Answered: yes, ISC, but not importable
   (unexported matrix) — fork-with-attribution path above.
2. xmrig: exact Zephyr algo flag; current GhostRider flag (`gr`).
3. Open-source status + stats API shapes of GPU miners per coin (lolMiner
   is closed-source — orchestration OK, redistribution not).
4. Current DAG/VRAM sizes (RVN/ETC-class grow) — `min_vram_mb` seeds are
   conservative.
5. Per-coin address checksum depth (regex MVP vs full checksum).
6. Pool public API shapes per recommended pool (payout detection).
