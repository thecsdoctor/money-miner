-- V2__seed_currencies.sql — the 20-coin catalog from the owner's research
-- doc (cryptocurrency_mining_research.md). Idempotent via ON CONFLICT.
--
-- STRICT POLICY: exactly these 20 mineable coins may exist in the catalog.
-- Non-mineable assets are excluded by policy. XCH is catalogued but
-- enabled=false / deferred (Proof-of-Space plotting UX is out of v0.1
-- scope; see dossier 01).
--
-- Pool seeds are best-effort stratum endpoints for Tier-1 coins; pool lists
-- are user-overridable in Settings -> Currencies, and the create-miner
-- wizard always lets the user confirm/edit pool_url. Verify pool endpoints
-- before pointing hashpower at them (docs/coins.md carries the full
-- research pool directory per coin).
--
-- adapter_config notes:
--   * sha256 {} is intentionally EMPTY: we never auto-download a miner
--     binary without a pinned per-platform checksum. Supply the binary by
--     path at setup (docs/installation.md), or record the vendor release
--     SHA-256 here first. See SECURITY.md.
--   * api_kind "xmrig" and "lolminer" have real stats parsers in the
--     adapter engine; any other api_kind reports stats as unavailable
--     (never fake numbers).
--   * ZEPH: exact xmrig RandomX variant flag is dossier open question #2 —
--     verify against current xmrig releases before enabling production ZEPH.

INSERT INTO currencies
  (symbol, name, algorithm, hardware_class, engine_strategy, adapter_config, min_vram_mb,
   explorer_tx_tpl, explorer_addr_tpl, homepage, pools, enabled, deferred_note)
VALUES
  ('XMR', 'Monero', 'randomx', 'cpu', 'adapter',
   '{"binary":"xmrig","version":"6.x","sha256":{},"args_template":["-o","{pool}","-u","{wallet}.{worker}","-p","x","--threads={threads}","--http-enabled","--http-port={api_port}","--algo=rx/0"],"api_kind":"xmrig","api_port":0,"release_url":"https://github.com/xmrig/xmrig/releases"}',
   NULL,
   'https://xmrchain.net/tx/{txid}', 'https://xmrchain.net/search?value={address}',
   'https://getmonero.org',
   '[{"name":"SupportXMR","url":"stratum+ssl://pool.supportxmr.com:443","api_tpl":"https://supportxmr.com/api/miner/{address}/stats","fee_pct":0.6},{"name":"p2pool","url":"stratum+tcp://p2pool.io:3333","api_tpl":"","fee_pct":0}]',
   true, NULL),

  ('ZEPH', 'Zephyr Protocol', 'randomx', 'cpu', 'adapter',
   '{"binary":"xmrig","version":"6.x","sha256":{},"args_template":["-o","{pool}","-u","{wallet}.{worker}","-p","x","--threads={threads}","--http-enabled","--http-port={api_port}","--algo=rx/0"],"api_kind":"xmrig","api_port":0,"release_url":"https://github.com/xmrig/xmrig/releases","note":"verify Zephyr RandomX variant flag against current xmrig (dossier open question #2)"}',
   NULL,
   'https://explorer.zephyrprotocol.com/tx/{txid}', 'https://explorer.zephyrprotocol.com/search?value={address}',
   'https://zephyrprotocol.com',
   '[{"name":"HeroMiners","url":"stratum+ssl://zephyr.herominers.com:1123","api_tpl":"https://zephyr.herominers.com/api/v1/miner/{address}/stats","fee_pct":0.9},{"name":"Woolypooly","url":"stratum+ssl://pool.woolypooly.com:3118","api_tpl":"","fee_pct":0.9}]',
   true, NULL),

  ('RTM', 'Raptoreum', 'ghostrider', 'cpu', 'adapter',
   '{"binary":"xmrig","version":"6.x","sha256":{},"args_template":["-o","{pool}","-u","{wallet}.{worker}","-p","x","--threads={threads}","--http-enabled","--http-port={api_port}","--algo=gr"],"api_kind":"xmrig","api_port":0,"release_url":"https://github.com/xmrig/xmrig/releases"}',
   NULL,
   'https://explorer.raptoreum.com/tx/{txid}', 'https://explorer.raptoreum.com/address/{address}',
   'https://raptoreum.com',
   '[{"name":"Flockpool","url":"stratum+ssl://eu.flockpool.com:4444","api_tpl":"https://api.flockpool.com/v1/wallet/rtm/{address}","fee_pct":1}]',
   true, NULL),

  ('KAS', 'Kaspa', 'kheavyhash', 'cpu+gpu', 'native-go',
   '{"binary":"lolMiner","version":"1.x","sha256":{},"args_template":["--algo","KASPA","--pool","{pool_host}","--port","{pool_port}","--user","{wallet}.{worker}","--pass","x","--apiport","{api_port}"],"api_kind":"lolminer","api_port":0,"release_url":"https://github.com/Lolliedieb/lolMiner-releases/releases","note":"closed-source: orchestration OK, redistribution NOT — binary must be user-supplied. Native pure-Go kHeavyHash engine is the preferred path; see engine_strategy."}',
   2048,
   'https://explorer.kaspa.org/txs/{txid}', 'https://explorer.kaspa.org/addresses/{address}',
   'https://kaspa.org',
   '[{"name":"2Miners","url":"stratum+tcp://kas.2miners.com:2020","api_tpl":"https://kas.2miners.com/api/accounts/{address}","fee_pct":1},{"name":"Woolypooly","url":"stratum+ssl://pool.woolypooly.com:3112","api_tpl":"","fee_pct":0.9}]',
   true, NULL),

  ('ERG', 'Ergo', 'autolykos2', 'gpu', 'adapter',
   '{"binary":"lolMiner","version":"1.x","sha256":{},"args_template":["--algo","AUTOLYKOS2","--pool","{pool_host}","--port","{pool_port}","--user","{wallet}.{worker}","--pass","x","--apiport","{api_port}"],"api_kind":"lolminer","api_port":0,"release_url":"https://github.com/Lolliedieb/lolMiner-releases/releases","note":"closed-source: orchestration OK, redistribution NOT — binary must be user-supplied"}',
   4096,
   'https://explorer.ergoplatform.com/en/transactions/{txid}', 'https://explorer.ergoplatform.com/en/addresses/{address}',
   'https://ergoplatform.org',
   '[{"name":"2Miners","url":"stratum+tcp://erg.2miners.com:8888","api_tpl":"https://erg.2miners.com/api/accounts/{address}","fee_pct":1},{"name":"HeroMiners","url":"stratum+ssl://ergo.herominers.com:1180","api_tpl":"","fee_pct":0.9}]',
   true, NULL),

  ('RVN', 'Ravencoin', 'kawpow', 'gpu', 'adapter',
   '{"binary":"kawpowminer","version":"1.2.x","sha256":{},"args_template":["-P","stratum+tcp://{wallet}.{worker}@{pool_host}:{pool_port}","--api-port","{api_port}"],"api_kind":"ethminer","api_port":0,"release_url":"https://github.com/RavenCommunity/kawpowminer/releases","note":"open source (GPL-3.0); DAG ~4 GB class — check current DAG size, grows over time (dossier open question #4)"}',
   4096,
   'https://ravencoin.network/tx/{txid}', 'https://ravencoin.network/address/{address}',
   'https://ravencoin.org',
   '[{"name":"2Miners","url":"stratum+tcp://rvn.2miners.com:6060","api_tpl":"https://rvn.2miners.com/api/accounts/{address}","fee_pct":1},{"name":"Flypool","url":"stratum+ssl://stratum-ravencoin.flypool.org:3443","api_tpl":"https://api-ravencoin.flypool.org/miner/{address}/dashboard","fee_pct":1}]',
   true, NULL),

  ('XEL', 'XELIS', 'xelis-hash', 'cpu+gpu', 'adapter',
   '{"binary":"xelis_miner","version":"2.x","sha256":{},"args_template":["--pool","{pool}","--wallet-address","{wallet}","--worker","{worker}","--threads","{threads}"],"api_kind":"unimplemented","api_port":0,"release_url":"https://github.com/xelis-project/xelis-blockchain/releases","note":"open source (Rust); stats parser not yet implemented — hashrate reported as unavailable, never faked"}',
   NULL,
   'https://explorer.xelis.io/txs/{txid}', 'https://explorer.xelis.io/addr/{address}',
   'https://xelis.io',
   '[{"name":"HeroMiners","url":"stratum+ssl://xelis.herominers.com:1225","api_tpl":"","fee_pct":0.9}]',
   true, NULL),

  ('VTC', 'Vertcoin', 'verthash', 'gpu', 'adapter',
   '{"binary":"VerthashMiner","version":"0.7.x","sha256":{},"args_template":["--pool","{pool}","--user","{wallet}.{worker}","--verthash-data","{verthash_dat}","--all-cuda-devices"],"api_kind":"unimplemented","api_port":0,"release_url":"https://github.com/CryptoGraphics/VerthashMiner/releases","note":"open source (GPL-3.0); requires verthash.dat (~1.2 GB) generated once per node"}',
   2048,
   'https://insight.vertcoin.org/tx/{txid}', 'https://insight.vertcoin.org/address/{address}',
   'https://vertcoin.org',
   '[{"name":"zpool","url":"stratum+tcp://verthash.mine.zpool.ca:6244","api_tpl":"","fee_pct":1.25}]',
   true, NULL),

  ('ETC', 'Ethereum Classic', 'etchash', 'gpu', 'adapter',
   '{"binary":"lolMiner","version":"1.x","sha256":{},"args_template":["--algo","ETCHASH","--pool","{pool_host}","--port","{pool_port}","--user","{wallet}.{worker}","--pass","x","--apiport","{api_port}"],"api_kind":"lolminer","api_port":0,"release_url":"https://github.com/Lolliedieb/lolMiner-releases/releases","note":"closed-source: orchestration OK, redistribution NOT — binary must be user-supplied"}',
   4096,
   'https://etcblockexplorer.com/tx/{txid}', 'https://etcblockexplorer.com/address/{address}',
   'https://ethereumclassic.org',
   '[{"name":"2Miners","url":"stratum+tcp://etc.2miners.com:1010","api_tpl":"https://etc.2miners.com/api/accounts/{address}","fee_pct":1},{"name":"Ethermine","url":"stratum+ssl://etc.ethermine.org:5555","api_tpl":"https://api.ethermine.org/miner/{address}/dashboard","fee_pct":1}]',
   true, NULL),

  ('FLUX', 'Flux', 'zelhash', 'gpu', 'adapter',
   '{"binary":"lolMiner","version":"1.x","sha256":{},"args_template":["--algo","FLUX","--pool","{pool_host}","--port","{pool_port}","--user","{wallet}.{worker}","--pass","x","--apiport","{api_port}"],"api_kind":"lolminer","api_port":0,"release_url":"https://github.com/Lolliedieb/lolMiner-releases/releases","note":"closed-source: orchestration OK, redistribution NOT — binary must be user-supplied"}',
   3072,
   'https://explorer.runonflux.io/tx/{txid}', 'https://explorer.runonflux.io/address/{address}',
   'https://runonflux.io',
   '[{"name":"2Miners","url":"stratum+tcp://flux.2miners.com:9090","api_tpl":"https://flux.2miners.com/api/accounts/{address}","fee_pct":1}]',
   true, NULL),

  ('ZEC', 'Zcash', 'equihash', 'gpu', 'adapter',
   '{"binary":"lolMiner","version":"1.x","sha256":{},"args_template":["--algo","EQUI192_7","--pool","{pool_host}","--port","{pool_port}","--user","{wallet}.{worker}","--pass","x","--apiport","{api_port}"],"api_kind":"lolminer","api_port":0,"release_url":"https://github.com/Lolliedieb/lolMiner-releases/releases","note":"closed-source: orchestration OK, redistribution NOT. Equihash is ASIC-dominated — GPU mining is hobby-scale; the UI states this honestly."}',
   2048,
   'https://blockchair.com/zcash/transaction/{txid}', 'https://blockchair.com/zcash/address/{address}',
   'https://z.cash',
   '[{"name":"2Miners","url":"stratum+tcp://zec.2miners.com:1010","api_tpl":"https://zec.2miners.com/api/accounts/{address}","fee_pct":1}]',
   true, NULL),

  ('ARRR', 'Pirate Chain', 'equihash-200-9', 'gpu', 'adapter',
   '{"binary":"lolMiner","version":"1.x","sha256":{},"args_template":["--algo","EQUI200_9","--pool","{pool_host}","--port","{pool_port}","--user","{wallet}.{worker}","--pass","x","--apiport","{api_port}"],"api_kind":"lolminer","api_port":0,"release_url":"https://github.com/Lolliedieb/lolMiner-releases/releases","note":"closed-source: orchestration OK, redistribution NOT"}',
   2048,
   'https://pirate.monster/tx/{txid}', 'https://pirate.monster/address/{address}',
   'https://piratechain.com',
   '[]',
   true, NULL),

  ('BTG', 'Bitcoin Gold', 'zhash', 'gpu', 'adapter',
   '{"binary":"lolMiner","version":"1.x","sha256":{},"args_template":["--algo","EQUI144_5","--pers","BgoldPoW","--pool","{pool_host}","--port","{pool_port}","--user","{wallet}.{worker}","--pass","x","--apiport","{api_port}"],"api_kind":"lolminer","api_port":0,"release_url":"https://github.com/Lolliedieb/lolMiner-releases/releases","note":"closed-source: orchestration OK, redistribution NOT"}',
   3072,
   'https://explorer.bitcoingold.org/insight/tx/{txid}', 'https://explorer.bitcoingold.org/insight/address/{address}',
   'https://bitcoingold.org',
   '[{"name":"2Miners","url":"stratum+tcp://btg.2miners.com:4040","api_tpl":"https://btg.2miners.com/api/accounts/{address}","fee_pct":1}]',
   true, NULL),

  ('GRIN', 'Grin', 'cuckaroo29', 'gpu', 'adapter',
   '{"binary":"grin-miner","version":"3.x","sha256":{},"args_template":[],"api_kind":"unimplemented","api_port":0,"release_url":"https://github.com/mimblewimble/grin-miner/releases","note":"open source (Rust); configured via grin-miner.toml rendered by the adapter — config_kind: toml. Cuckatoo32+ targets larger hardware.","config_kind":"grin-miner.toml"}',
   6144,
   'https://grinscan.net/kernel/{txid}', 'https://grinscan.net/',
   'https://grin.mw',
   '[{"name":"2Miners","url":"stratum+tcp://grin.2miners.com:3030","api_tpl":"https://grin.2miners.com/api/accounts/{address}","fee_pct":1}]',
   true, NULL),

  ('BEAM', 'Beam', 'beamhash3', 'gpu', 'adapter',
   '{"binary":"lolMiner","version":"1.x","sha256":{},"args_template":["--algo","BEAM-III","--pool","{pool_host}","--port","{pool_port}","--user","{wallet}.{worker}","--pass","x","--apiport","{api_port}"],"api_kind":"lolminer","api_port":0,"release_url":"https://github.com/Lolliedieb/lolMiner-releases/releases","note":"closed-source: orchestration OK, redistribution NOT"}',
   3072,
   'https://explorer.beam.mw/search?q={txid}', 'https://explorer.beam.mw/',
   'https://beam.mw',
   '[{"name":"HeroMiners","url":"stratum+ssl://beam.herominers.com:1030","api_tpl":"","fee_pct":0.9}]',
   true, NULL),

  ('CLORE', 'Clore.ai', 'kawpow', 'gpu', 'adapter',
   '{"binary":"kawpowminer","version":"1.2.x","sha256":{},"args_template":["-P","stratum+tcp://{wallet}.{worker}@{pool_host}:{pool_port}","--api-port","{api_port}"],"api_kind":"ethminer","api_port":0,"release_url":"https://github.com/RavenCommunity/kawpowminer/releases","note":"open source (GPL-3.0); shares the RVN KAWPOW adapter"}',
   4096,
   'https://explorer.clore.ai/tx/{txid}', 'https://explorer.clore.ai/address/{address}',
   'https://clore.ai',
   '[{"name":"HeroMiners","url":"stratum+ssl://clore.herominers.com:1163","api_tpl":"","fee_pct":0.9}]',
   true, NULL),

  ('OCTA', 'OctaSpace', 'ethash-variant', 'gpu', 'adapter',
   '{"binary":"lolMiner","version":"1.x","sha256":{},"args_template":["--algo","ETHASH","--pool","{pool_host}","--port","{pool_port}","--user","{wallet}.{worker}","--pass","x","--apiport","{api_port}"],"api_kind":"lolminer","api_port":0,"release_url":"https://github.com/Lolliedieb/lolMiner-releases/releases","note":"closed-source: orchestration OK, redistribution NOT"}',
   4096,
   'https://explorer.octa.space/tx/{txid}', 'https://explorer.octa.space/address/{address}',
   'https://octa.space',
   '[]',
   true, NULL),

  ('DASH', 'Dash', 'x11', 'cpu+gpu', 'adapter',
   '{"binary":"","version":"","sha256":{},"args_template":[],"api_kind":"unimplemented","api_port":0,"note":"X11 is ASIC-dominated — home CPU/GPU mining is not competitive. Adapter config intentionally empty; set a pool + binary override in Settings if you know what you are doing. The UI states this honestly."}',
   NULL,
   'https://blockchair.com/dash/transaction/{txid}', 'https://blockchair.com/dash/address/{address}',
   'https://dash.org',
   '[]',
   true, NULL),

  ('DGB', 'DigiByte', 'skein', 'cpu+gpu', 'adapter',
   '{"binary":"ccminer","version":"2.x","sha256":{},"args_template":["-a","skein","-o","{pool}","-u","{wallet}.{worker}","-p","x","--api-bind","{api_port}"],"api_kind":"ccminer","api_port":0,"note":"multi-algo chain; Skein/Qubit are the CPU/GPU-friendly lanes (research doc). ccminer-class adapter is tier-3 data, not yet exercised end-to-end."}',
   NULL,
   'https://digiexplorer.info/tx/{txid}', 'https://digiexplorer.info/address/{address}',
   'https://digibyte.org',
   '[]',
   true, NULL),

  ('XCH', 'Chia', 'proof-of-space-and-time', 'storage', 'deferred',
   NULL, NULL,
   'https://spacescan.io/txns/{txid}', 'https://spacescan.io/address/{address}',
   'https://chia.net',
   '[]',
   false, 'Proof-of-Space-and-Time needs a plotting UX (hundreds of GB plot files), out of v0.1 scope. Catalogued for completeness per the research doc; cannot be selected for miner creation.')
ON CONFLICT (symbol) DO UPDATE SET
  name = EXCLUDED.name,
  algorithm = EXCLUDED.algorithm,
  hardware_class = EXCLUDED.hardware_class,
  engine_strategy = EXCLUDED.engine_strategy,
  adapter_config = EXCLUDED.adapter_config,
  min_vram_mb = EXCLUDED.min_vram_mb,
  explorer_tx_tpl = EXCLUDED.explorer_tx_tpl,
  explorer_addr_tpl = EXCLUDED.explorer_addr_tpl,
  homepage = EXCLUDED.homepage,
  pools = EXCLUDED.pools,
  enabled = EXCLUDED.enabled,
  deferred_note = EXCLUDED.deferred_note;
