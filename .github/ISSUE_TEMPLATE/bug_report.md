---
name: Bug report
about: Something is broken
title: "[bug] "
labels: bug
---

**What happened** (and what you expected instead)

**Steps to reproduce**

**Environment**
- money-miner version / commit:
- server OS/arch:
- worker OS/arch (if swarm-related):
- miner binary + version (if engine-related, e.g. xmrig 6.x):

**Logs** — server logs (`docker logs money-miner-server-1`), worker stderr,
or browser console. Redact tokens, wallet addresses you don't want public,
and any secrets.

**Honesty check** — if the bug involves numbers that looked wrong (hashrate,
shares, payouts), say where you verified the truth (pool dashboard,
explorer). We never fake numbers; wrong ones are treated as high priority.
