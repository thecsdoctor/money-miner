// Typed API client for the money-miner /v1 surface (via /api at the edge).
// Bearer from the in-memory auth module; error envelope per dossier 04.
import { ensureFresh, getAccessToken } from "../auth/keycloak";
import { env } from "../env";

export class ApiError extends Error {
  code: string;
  status: number;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  await ensureFresh();
  const token = getAccessToken();
  const headers: Record<string, string> = {};
  if (token) headers["Authorization"] = `Bearer ${token}`;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const resp = await fetch(`${env.API_URL}${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (resp.status === 204) return undefined as T;
  const text = await resp.text();
  let data: unknown = undefined;
  try {
    data = text ? JSON.parse(text) : undefined;
  } catch {
    /* non-JSON */
  }
  if (!resp.ok) {
    const e = (data as { error?: { code?: string; message?: string } })?.error;
    throw new ApiError(resp.status, e?.code ?? "http_" + resp.status, e?.message ?? resp.statusText);
  }
  return data as T;
}

export const api = {
  get: <T>(path: string) => request<T>("GET", path),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, body),
  patch: <T>(path: string, body?: unknown) => request<T>("PATCH", path, body),
  put: <T>(path: string, body?: unknown) => request<T>("PUT", path, body),
  del: <T>(path: string) => request<T>("DELETE", path),
};

// ---- SSE (fetch-based: EventSource cannot set the Authorization header) ----

export interface SseEvent {
  type: string;
  data: unknown;
}

// subscribeEvents opens /v1/events and invokes onEvent per frame. Returns a
// close function. Auto-reconnect is the caller's loop (pages wrap this).
export function subscribeEvents(onEvent: (ev: SseEvent) => void, onError?: (e: unknown) => void): () => void {
  const ctrl = new AbortController();
  (async () => {
    try {
      await ensureFresh();
      const token = getAccessToken();
      const resp = await fetch(`${env.API_URL}/events`, {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
        signal: ctrl.signal,
      });
      if (!resp.ok || !resp.body) throw new ApiError(resp.status, "sse", "event stream failed");
      const reader = resp.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        let idx;
        while ((idx = buf.indexOf("\n\n")) >= 0) {
          const frame = buf.slice(0, idx);
          buf = buf.slice(idx + 2);
          let type = "message";
          let data = "";
          for (const line of frame.split("\n")) {
            if (line.startsWith("event:")) type = line.slice(6).trim();
            if (line.startsWith("data:")) data += line.slice(5).trim();
          }
          if (data) {
            try {
              onEvent({ type, data: JSON.parse(data) });
            } catch {
              /* ignore malformed frame */
            }
          }
        }
      }
    } catch (e) {
      if (!ctrl.signal.aborted) onError?.(e);
    }
  })();
  return () => ctrl.abort();
}

// ---- API types (hand-written; `make -C money-miner-api/types` regenerates
// types.generated.ts from the OpenAPI contract of record for reference) ----

export interface PoolEntry {
  name: string;
  url: string;
  api_tpl: string;
  fee_pct: number;
}

export interface Currency {
  id: string;
  symbol: string;
  name: string;
  algorithm: string;
  hardware_class: string;
  engine_strategy: string;
  adapter_config?: Record<string, unknown> | null;
  min_vram_mb: number | null;
  explorer_tx_tpl: string;
  explorer_addr_tpl: string;
  homepage: string;
  pools: PoolEntry[];
  enabled: boolean;
  deferred_note?: string;
}

export interface Wallet {
  id: string;
  currency: string;
  address: string;
  label: string;
  validated: boolean;
  created_at: string;
}

export interface Miner {
  id: string;
  name: string;
  currency: string;
  wallet_id: string | null;
  engine: string;
  simulated: boolean;
  pool_url: string;
  cpu_pct: number;
  gpu_pct: number;
  status: string;
  hashrate: number;
  shares_accepted: number;
  shares_rejected: number;
  created_at: string;
  updated_at: string;
}

export interface MinerRun {
  id: string;
  miner_id: string;
  started_at: string;
  stopped_at: string | null;
  stop_reason: string | null;
  avg_hashrate: number;
  shares_accepted: number;
  shares_rejected: number;
}

export interface MinerDetail extends Miner {
  current_run: MinerRun | null;
  workers: Worker[];
}

export interface Worker {
  id: string;
  name: string;
  kind: string;
  os: string;
  arch: string;
  cpu_model: string;
  cpu_cores: number;
  gpu_model: string;
  vram_mb: number;
  caps?: Record<string, unknown>;
  status: string;
  hashrate: number;
  currency: string;
  last_seen: string | null;
  enrolled_at: string;
}

export interface JoinCode {
  id: string;
  expires_at: string;
  consumed: boolean;
  created_at: string;
}

export interface NewJoinCode {
  id: string;
  code: string;
  expires_at: string;
  enroll_command: string;
}

export interface OrbWorker {
  id: string;
  name: string;
  kind: string;
  status: string;
  hashrate: number;
  currency: string;
}

export interface SwarmStats {
  workers: OrbWorker[];
  totals: {
    workers_online: number;
    workers_total: number;
    hashrate: number;
    shares_24h?: number;
    payouts_24h?: number;
  };
}

export interface Payout {
  id: string;
  currency: string;
  amount: number;
  txid: string;
  explorer_url: string;
  source_pool: string;
  verified: boolean;
  paid_at: string | null;
  detected_at: string;
}

export interface MetricPoint {
  ts: string;
  hashrate: number;
  shares_accepted: number;
  shares_rejected: number;
}

export interface CurrencySetting {
  currency: string;
  pool_url: string;
  enabled: boolean;
  custom_adapter?: Record<string, unknown>;
}

export interface ExchangeSetting {
  exchange: string;
  currencies: string[];
  has_api_key: boolean;
}

export interface JoinInfo {
  valid: boolean;
  app_name: string;
  app_domain: string;
  browser_mining: { available: boolean; reason: string };
}

// hashrate formatting (mono, tabular — no jitter)
export function fmtHashrate(h: number): string {
  if (!h || h <= 0) return "0 H/s";
  const units = ["H/s", "kH/s", "MH/s", "GH/s", "TH/s", "PH/s"];
  let u = 0;
  let v = h;
  while (v >= 1000 && u < units.length - 1) {
    v /= 1000;
    u++;
  }
  return `${v >= 100 ? v.toFixed(0) : v >= 10 ? v.toFixed(1) : v.toFixed(2)} ${units[u]}`;
}
