-- V1__init.sql — money-miner schema (dossier 03).
-- Extensions are also ensured by create-database.sh; kept here so a bare
-- restore + flyway migrate yields a working schema.

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE currencies (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  symbol text UNIQUE NOT NULL,            -- XMR
  name text NOT NULL,                     -- Monero
  algorithm text NOT NULL,                -- randomx
  hardware_class text NOT NULL,           -- cpu|gpu|cpu+gpu|storage
  engine_strategy text NOT NULL,          -- adapter|native-go|browser-wasm|deferred
  adapter_config jsonb,                   -- see 01-algorithms.md
  min_vram_mb int,
  explorer_tx_tpl text NOT NULL,          -- https://xmrchain.net/tx/{txid}
  explorer_addr_tpl text NOT NULL,
  homepage text NOT NULL,
  pools jsonb NOT NULL DEFAULT '[]',      -- [{name,url,api_tpl,fee_pct}]
  enabled boolean NOT NULL DEFAULT true,  -- false for deferred (XCH)
  deferred_note text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE wallets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_sub text NOT NULL,
  currency_id uuid NOT NULL REFERENCES currencies(id),
  address text NOT NULL,
  label text,
  validated boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(owner_sub, currency_id)          -- one wallet per currency per user
);

CREATE TABLE miners (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_sub text NOT NULL,
  name text NOT NULL,
  currency_id uuid NOT NULL REFERENCES currencies(id),
  wallet_id uuid REFERENCES wallets(id),
  engine text NOT NULL,                   -- adapter|native-go|simulated
  pool_url text NOT NULL,
  cpu_pct int NOT NULL DEFAULT 50 CHECK (cpu_pct BETWEEN 1 AND 100),
  gpu_pct int NOT NULL DEFAULT 0 CHECK (gpu_pct BETWEEN 0 AND 100),
  status text NOT NULL DEFAULT 'created', -- created|queued|running|stopping|stopped|error
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX miners_owner_idx ON miners (owner_sub);

CREATE TABLE miner_runs (                 -- history: one row per start→stop session
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  miner_id uuid NOT NULL REFERENCES miners(id) ON DELETE CASCADE,
  started_at timestamptz NOT NULL DEFAULT now(),
  stopped_at timestamptz,
  stop_reason text,                       -- user|error|revoked
  avg_hashrate numeric,
  shares_accepted bigint DEFAULT 0,
  shares_rejected bigint DEFAULT 0
);
CREATE INDEX miner_runs_miner_idx ON miner_runs (miner_id, started_at DESC);

CREATE TABLE workers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_sub text NOT NULL,
  name text NOT NULL,
  kind text NOT NULL,                     -- native|browser|master
  os text, arch text,
  cpu_model text, cpu_cores int,
  gpu_model text, vram_mb int,
  caps jsonb,                             -- {engines:[...], max_threads:n}
  token_hash text NOT NULL,               -- sha256(worker_token)
  status text NOT NULL DEFAULT 'enrolled',-- enrolled|connected|assigned|mining|idle|offline|revoked
  last_seen timestamptz,
  enrolled_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz
);
CREATE INDEX workers_owner_idx ON workers (owner_sub);

CREATE TABLE join_codes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_sub text NOT NULL,
  code_hash text NOT NULL,                -- bcrypt(code); plaintext shown once, never stored
  expires_at timestamptz NOT NULL,
  consumed_by uuid REFERENCES workers(id),
  consumed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX join_codes_owner_open_idx ON join_codes (owner_sub) WHERE consumed_by IS NULL;

CREATE TABLE swarm_jobs (                 -- assignment history (audit + orb replay)
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  miner_id uuid NOT NULL REFERENCES miners(id) ON DELETE CASCADE,
  worker_id uuid NOT NULL REFERENCES workers(id) ON DELETE CASCADE,
  assigned_at timestamptz NOT NULL DEFAULT now(),
  ended_at timestamptz,
  status text NOT NULL DEFAULT 'assigned' -- assigned|ack|ended|error
);
CREATE INDEX swarm_jobs_miner_idx ON swarm_jobs (miner_id);
CREATE INDEX swarm_jobs_worker_idx ON swarm_jobs (worker_id);

CREATE TABLE metric_samples (             -- raw telemetry; 48 h retention
  miner_id uuid NOT NULL REFERENCES miners(id) ON DELETE CASCADE,
  worker_id uuid REFERENCES workers(id) ON DELETE SET NULL,
  ts timestamptz NOT NULL DEFAULT now(),
  hashrate numeric NOT NULL,              -- H/s normalized
  shares_accepted int NOT NULL DEFAULT 0,
  shares_rejected int NOT NULL DEFAULT 0
);
CREATE INDEX metric_samples_miner_ts_idx ON metric_samples (miner_id, ts DESC);

CREATE TABLE metric_rollups (             -- hourly aggregates, kept forever
  miner_id uuid NOT NULL REFERENCES miners(id) ON DELETE CASCADE,
  bucket timestamptz NOT NULL,            -- date_trunc('hour', ts)
  avg_hashrate numeric, max_hashrate numeric,
  shares_accepted bigint, shares_rejected bigint,
  PRIMARY KEY (miner_id, bucket)
);

CREATE TABLE blocks_found (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  miner_id uuid REFERENCES miners(id) ON DELETE SET NULL,
  currency_id uuid NOT NULL REFERENCES currencies(id),
  height bigint, hash text,
  found_at timestamptz, source text,      -- pool-api|explorer
  explorer_url text
);

CREATE TABLE payouts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_sub text NOT NULL,
  currency_id uuid NOT NULL REFERENCES currencies(id),
  wallet_id uuid REFERENCES wallets(id),
  amount numeric NOT NULL,
  txid text,
  explorer_url text,
  source_pool text,
  verified boolean NOT NULL DEFAULT false,
  paid_at timestamptz,
  detected_at timestamptz NOT NULL DEFAULT now(),
  raw jsonb,
  UNIQUE(currency_id, txid)
);
CREATE INDEX payouts_owner_idx ON payouts (owner_sub, detected_at DESC);

CREATE TABLE settings (                   -- app settings KV per user
  owner_sub text NOT NULL,
  key text NOT NULL,                      -- theme|language|electricity_cost|...
  value jsonb NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (owner_sub, key)
);

CREATE TABLE currency_settings (          -- per-currency overrides per user
  owner_sub text NOT NULL,
  currency_id uuid NOT NULL REFERENCES currencies(id),
  pool_url text, custom_adapter jsonb, enabled boolean DEFAULT true,
  PRIMARY KEY (owner_sub, currency_id)
);

CREATE TABLE exchange_settings (          -- price/rate preferences per user
  owner_sub text NOT NULL,
  exchange text NOT NULL,                 -- kraken|binance|...
  currencies text[] NOT NULL DEFAULT '{}',
  api_key_enc bytea,                      -- pgcrypto-encrypted, read-only keys only; optional
  PRIMARY KEY (owner_sub, exchange)
);
