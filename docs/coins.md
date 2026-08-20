# Supported Coins

Exactly the 20 research-verified mineable assets from the owner's research
document exist in this catalog — non-mineable assets are excluded by policy.
This page is generated from the seeded catalog (`deploy/migrations/V2__seed_currencies.sql`).

| Coin | Name | Algorithm | Hardware | Strategy | Min. VRAM | Pools (seed) | Explorer tx template |
| --- | --- | --- | --- | --- | --- | --- | --- |
| **ARRR** | [Pirate Chain](https://piratechain.com) | equihash-200-9 | gpu | adapter | 2048 | — | `https://pirate.monster/tx/{txid}` |
| **BEAM** | [Beam](https://beam.mw) | beamhash3 | gpu | adapter | 3072 | HeroMiners | `https://explorer.beam.mw/search?q={txid}` |
| **BTG** | [Bitcoin Gold](https://bitcoingold.org) | zhash | gpu | adapter | 3072 | 2Miners | `https://explorer.bitcoingold.org/insight/tx/{txid}` |
| **CLORE** | [Clore.ai](https://clore.ai) | kawpow | gpu | adapter | 4096 | HeroMiners | `https://explorer.clore.ai/tx/{txid}` |
| **DASH** | [Dash](https://dash.org) | x11 | cpu+gpu | adapter | — | — | `https://blockchair.com/dash/transaction/{txid}` |
| **DGB** | [DigiByte](https://digibyte.org) | skein | cpu+gpu | adapter | — | — | `https://digiexplorer.info/tx/{txid}` |
| **ERG** | [Ergo](https://ergoplatform.org) | autolykos2 | gpu | adapter | 4096 | 2Miners, HeroMiners | `https://explorer.ergoplatform.com/en/transactions/{txid}` |
| **ETC** | [Ethereum Classic](https://ethereumclassic.org) | etchash | gpu | adapter | 4096 | 2Miners, Ethermine | `https://etcblockexplorer.com/tx/{txid}` |
| **FLUX** | [Flux](https://runonflux.io) | zelhash | gpu | adapter | 3072 | 2Miners | `https://explorer.runonflux.io/tx/{txid}` |
| **GRIN** | [Grin](https://grin.mw) | cuckaroo29 | gpu | adapter | 6144 | 2Miners | `https://grinscan.net/kernel/{txid}` |
| **KAS** | [Kaspa](https://kaspa.org) | kheavyhash | cpu+gpu | native-go | 2048 | 2Miners, Woolypooly | `https://explorer.kaspa.org/txs/{txid}` |
| **OCTA** | [OctaSpace](https://octa.space) | ethash-variant | gpu | adapter | 4096 | — | `https://explorer.octa.space/tx/{txid}` |
| **RTM** | [Raptoreum](https://raptoreum.com) | ghostrider | cpu | adapter | — | Flockpool | `https://explorer.raptoreum.com/tx/{txid}` |
| **RVN** | [Ravencoin](https://ravencoin.org) | kawpow | gpu | adapter | 4096 | 2Miners, Flypool | `https://ravencoin.network/tx/{txid}` |
| **VTC** | [Vertcoin](https://vertcoin.org) | verthash | gpu | adapter | 2048 | zpool | `https://insight.vertcoin.org/tx/{txid}` |
| **XCH** **(deferred)** | [Chia](https://chia.net) | proof-of-space-and-time | storage | deferred | — | — | `https://spacescan.io/txns/{txid}` |
| **XEL** | [XELIS](https://xelis.io) | xelis-hash | cpu+gpu | adapter | — | HeroMiners | `https://explorer.xelis.io/txs/{txid}` |
| **XMR** | [Monero](https://getmonero.org) | randomx | cpu | adapter | — | SupportXMR, p2pool | `https://xmrchain.net/tx/{txid}` |
| **ZEC** | [Zcash](https://z.cash) | equihash | gpu | adapter | 2048 | 2Miners | `https://blockchair.com/zcash/transaction/{txid}` |
| **ZEPH** | [Zephyr Protocol](https://zephyrprotocol.com) | randomx | cpu | adapter | — | HeroMiners, Woolypooly | `https://explorer.zephyrprotocol.com/tx/{txid}` |

## Notes

- **XCH (Chia)** is catalogued but `enabled=false` / deferred: Proof-of-Space-and-Time
  needs a plotting UX (hundreds of GB plot files), out of v0.1 scope. It cannot be
  selected for miner creation (`currency_deferred`).
- **Pool seeds are best-effort endpoints** for getting started; the create-miner
  wizard always lets you confirm or override `pool_url`, and Settings → Currencies
  carries a per-user override. Verify a pool's endpoint and terms before pointing
  hashpower at it. The research doc's full pool directory per coin:
  SupportXMR/p2pool (XMR), Ethermine/2Miners/ViaBTC (ETC), 2Miners/Flypool/Nanopool (RVN),
  HeroMiners/2Miners/Woolypooly (ERG), official XELIS pool/HeroMiners (XEL),
  MiningPoolHub/zpool/OCM (VTC), HeroMiners/Woolypooly (ZEPH), Flockpool/RTM-Pool (RTM),
  HeroMiners/Rplant (CLORE), Minerpool/EnigmaPool/FluxPool (FLUX), Woolypooly/Maxiopool (OCTA),
  SpacePool/Flexpool/AKPool (XCH), ViaBTC/2Miners/SlushPool (ZEC), ViaBTC/F2Pool (DASH),
  Pirate Pool/MiningPoolHub (ARRR), Hummerpool/Woolypooly/2Miners (KAS),
  MiningPoolHub/DGB Pool (DGB), 2Miners/Suprnova (BTG), Grinmint/2Miners (GRIN),
  Leafpool/2Miners/Woolypooly (BEAM).
- **Adapter binaries are never bundled.** Download from the vendor's official GitHub
  releases (xmrig, lolMiner, kawpowminer, VerthashMiner, grin-miner, xelis-miner),
  verify the SHA-256, and pin it in the currency's `adapter_config` (see SECURITY.md).
  lolMiner is closed-source: orchestration is fine, redistribution is not.
- **VRAM gates** (`min_vram_mb`) are conservative v0.1 seeds — DAG sizes grow over
  time. The UI enforces them and says why when a GPU coin is not viable on a worker.
- **DASH/ZEC/DGB** carry ASIC-era honesty notes: home hardware is not competitive
  there; they are catalogued for completeness, not encouragement.
