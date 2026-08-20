import React, { useCallback, useEffect, useState } from "react";
import {
  api,
  ApiError,
  type Currency,
  type CurrencySetting,
  type ExchangeSetting,
  type JoinCode,
  type NewJoinCode,
  type Wallet,
  type Worker,
} from "../api/client";
import { JoinCodeDialog } from "../components/JoinCodeDialog";
import { EmptyState, StatusDot } from "../components/StatCard";
import { LangSwitcher, ThemeToggle } from "../components/Toggles";
import { WalletForm } from "../components/WalletForm";
import { useT } from "../i18n";

// Settings — tabs: General · Swarm · Wallets · Exchanges · Currencies (dossier 05).
export function SettingsPage() {
  const { t } = useT();
  const [tab, setTab] = useState(0);
  const tabs = [
    "settings.tabs.general",
    "settings.tabs.swarm",
    "settings.tabs.wallets",
    "settings.tabs.exchanges",
    "settings.tabs.currencies",
  ];
  return (
    <div>
      <div className="page-head">
        <h1>{t("settings.title")}</h1>
      </div>
      <div className="tabs">
        {tabs.map((key, i) => (
          <button key={key} className={i === tab ? "active" : ""} onClick={() => setTab(i)}>
            {t(key)}
          </button>
        ))}
      </div>
      {tab === 0 ? <GeneralTab /> : null}
      {tab === 1 ? <SwarmTab /> : null}
      {tab === 2 ? <WalletsTab /> : null}
      {tab === 3 ? <ExchangesTab /> : null}
      {tab === 4 ? <CurrenciesTab /> : null}
    </div>
  );
}

// ---------- General ----------

function GeneralTab() {
  const { t } = useT();
  const [cost, setCost] = useState("");
  const [saved, setSaved] = useState(false);
  useEffect(() => {
    api.get<Record<string, unknown>>("/settings").then((s) => {
      if (typeof s["electricity_cost"] === "number" || typeof s["electricity_cost"] === "string") {
        setCost(String(s["electricity_cost"]));
      }
    }).catch(() => {});
  }, []);
  const save = async () => {
    await api.put("/settings/electricity_cost", { value: cost });
    setSaved(true);
    setTimeout(() => setSaved(false), 1500);
  };
  return (
    <div className="card" style={{ maxWidth: 560 }}>
      <label className="field">
        <span className="field-label">{t("settings.general.theme")}</span>
        <div>
          <ThemeToggle />
        </div>
      </label>
      <label className="field">
        <span className="field-label">{t("settings.general.language")}</span>
        <div>
          <LangSwitcher />
        </div>
      </label>
      <label className="field">
        <span className="field-label">{t("settings.general.electricity_cost")}</span>
        <input type="text" className="mono" value={cost} onChange={(e) => setCost(e.target.value)} placeholder="0.25" />
        <div className="hint">{t("settings.general.electricity_hint")}</div>
      </label>
      <button className="primary" onClick={save}>
        {t("common.save")}
      </button>{" "}
      {saved ? <span className="hint">{t("settings.saved")}</span> : null}
    </div>
  );
}

// ---------- Swarm ----------

function SwarmTab() {
  const { t } = useT();
  const [codes, setCodes] = useState<JoinCode[]>([]);
  const [workers, setWorkers] = useState<Worker[]>([]);
  const [fresh, setFresh] = useState<NewJoinCode | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    api.get<{ items: JoinCode[] }>("/swarm/join-codes").then((d) => setCodes(d.items)).catch(() => {});
    api.get<{ items: Worker[] }>("/workers").then((d) => setWorkers(d.items)).catch((e) => setError(e.message));
  }, []);
  useEffect(() => {
    load();
    const iv = setInterval(load, 10_000);
    return () => clearInterval(iv);
  }, [load]);

  const generate = async () => {
    try {
      const c = await api.post<NewJoinCode>("/swarm/join-codes");
      setFresh(c);
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e));
    }
  };
  const revoke = async (w: Worker) => {
    if (!window.confirm(t("settings.swarm.revoke_confirm"))) return;
    await api.post(`/workers/${w.id}/revoke`);
    load();
  };
  const deleteCode = async (id: string) => {
    await api.del(`/swarm/join-codes/${id}`);
    load();
  };

  return (
    <div>
      {error ? <div className="error-banner">{error}</div> : null}
      <div className="card">
        <button className="primary" onClick={generate}>
          {t("settings.swarm.generate")}
        </button>{" "}
        <span className="hint">{t("settings.swarm.code_expires", { minutes: 15 })}</span>
        <p className="hint" style={{ marginTop: "var(--sp-3)" }}>
          {t("settings.swarm.join_page_hint")}{" "}
          <a href="/join" target="_blank" rel="noreferrer">
            {window.location.origin}/join
          </a>
        </p>
      </div>

      <h3>{t("settings.swarm.active_codes")}</h3>
      <div className="card">
        {codes.filter((c) => !c.consumed).length === 0 ? (
          <p className="hint">{t("settings.swarm.no_codes")}</p>
        ) : (
          <table className="data">
            <tbody>
              {codes
                .filter((c) => !c.consumed)
                .map((c) => (
                  <tr key={c.id}>
                    <td className="mono">{c.id.slice(0, 8)}…</td>
                    <td className="mono">{new Date(c.expires_at).toLocaleTimeString()}</td>
                    <td>
                      <button className="small danger" onClick={() => deleteCode(c.id)}>
                        {t("common.delete")}
                      </button>
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        )}
      </div>

      <h3>{t("settings.swarm.workers")}</h3>
      <div className="card">
        {workers.length === 0 ? (
          <EmptyState title={t("empty.workers.title")} actionLabel={t("empty.workers.action")} onAction={generate} />
        ) : (
          <table className="data">
            <thead>
              <tr>
                <th>{t("settings.swarm.col.name")}</th>
                <th>{t("settings.swarm.col.kind")}</th>
                <th>{t("settings.swarm.col.status")}</th>
                <th>{t("settings.swarm.col.hardware")}</th>
                <th>{t("settings.swarm.col.last_seen")}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {workers.map((w) => (
                <tr key={w.id}>
                  <td>{w.name}</td>
                  <td>
                    <span className="chip">{w.kind}</span>
                  </td>
                  <td>
                    <StatusDot status={w.status} />
                    {w.status}
                  </td>
                  <td className="mono" style={{ fontSize: "var(--fs-1)" }}>
                    {w.cpu_model ? `${w.cpu_cores}C` : ""} {w.gpu_model ? `+ ${w.gpu_model}` : ""}
                  </td>
                  <td className="mono">{w.last_seen ? new Date(w.last_seen).toLocaleString() : t("common.never")}</td>
                  <td>
                    {w.status !== "revoked" ? (
                      <button className="small danger" onClick={() => revoke(w)}>
                        {t("settings.swarm.revoke")}
                      </button>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      {fresh ? <JoinCodeDialog code={fresh} onClose={() => setFresh(null)} /> : null}
    </div>
  );
}

// ---------- Wallets ----------

function WalletsTab() {
  const { t } = useT();
  const [wallets, setWallets] = useState<Wallet[]>([]);
  const [currencies, setCurrencies] = useState<Currency[]>([]);
  const load = useCallback(() => {
    api.get<{ items: Wallet[] }>("/wallets").then((d) => setWallets(d.items)).catch(() => {});
    api.get<{ items: Currency[] }>("/currencies").then((d) => setCurrencies(d.items)).catch(() => {});
  }, []);
  useEffect(load, [load]);

  const remove = async (id: string) => {
    if (!window.confirm(t("settings.wallets.delete_confirm"))) return;
    await api.del(`/wallets/${id}`);
    load();
  };

  return (
    <div>
      <div className="card">
        {wallets.length === 0 ? (
          <p className="hint">{t("settings.wallets.empty")}</p>
        ) : (
          <table className="data">
            <thead>
              <tr>
                <th>{t("settings.wallets.col.currency")}</th>
                <th>{t("settings.wallets.col.address")}</th>
                <th>{t("settings.wallets.col.label")}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {wallets.map((w) => (
                <tr key={w.id}>
                  <td>
                    <span className="chip">{w.currency}</span>
                  </td>
                  <td className="mono" style={{ fontSize: "var(--fs-1)", wordBreak: "break-all" }}>
                    {w.address}
                  </td>
                  <td>{w.label}</td>
                  <td>
                    <button className="small danger" onClick={() => remove(w.id)}>
                      {t("common.delete")}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      <WalletForm currencies={currencies} exclude={wallets.map((w) => w.currency)} onSaved={load} />
    </div>
  );
}

// ---------- Exchanges ----------

function ExchangesTab() {
  const { t } = useT();
  const [items, setItems] = useState<ExchangeSetting[]>([]);
  const [exchange, setExchange] = useState("kraken");
  const [symbols, setSymbols] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    api.get<{ items: ExchangeSetting[] }>("/exchange-settings").then((d) => setItems(d.items)).catch(() => {});
  }, []);
  useEffect(load, [load]);

  const save = async () => {
    setError(null);
    try {
      await api.put(`/exchange-settings/${exchange}`, {
        currencies: symbols.split(",").map((s) => s.trim()).filter(Boolean),
        api_key: apiKey || undefined,
      });
      setApiKey("");
      setSaved(true);
      setTimeout(() => setSaved(false), 1500);
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e));
    }
  };

  return (
    <div>
      <div className="notice">{t("settings.exchanges.note")}</div>
      <div className="card" style={{ marginTop: "var(--sp-4)" }}>
        {items.length === 0 ? (
          <p className="hint">{t("settings.exchanges.empty")}</p>
        ) : (
          <table className="data">
            <tbody>
              {items.map((e) => (
                <tr key={e.exchange}>
                  <td>{e.exchange}</td>
                  <td className="mono">{e.currencies.join(", ")}</td>
                  <td className="hint">{e.has_api_key ? t("settings.exchanges.has_key") : ""}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      <div className="card" style={{ marginTop: "var(--sp-4)", maxWidth: 560 }}>
        <label className="field">
          <span className="field-label">{t("settings.exchanges.exchange")}</span>
          <input type="text" value={exchange} onChange={(e) => setExchange(e.target.value)} className="mono" />
        </label>
        <label className="field">
          <span className="field-label">{t("settings.exchanges.currencies")}</span>
          <input type="text" value={symbols} onChange={(e) => setSymbols(e.target.value)} placeholder="XMR, KAS" className="mono" />
        </label>
        <label className="field">
          <span className="field-label">{t("settings.exchanges.api_key")}</span>
          <input type="password" value={apiKey} onChange={(e) => setApiKey(e.target.value)} autoComplete="off" />
          <div className="hint">{t("settings.exchanges.key_note")}</div>
        </label>
        {error ? <div className="error-banner">{error}</div> : null}
        <button className="primary" onClick={save}>
          {t("common.save")}
        </button>{" "}
        {saved ? <span className="hint">{t("settings.saved")}</span> : null}
      </div>
    </div>
  );
}

// ---------- Currencies ----------

function CurrenciesTab() {
  const { t } = useT();
  const [currencies, setCurrencies] = useState<Currency[]>([]);
  const [overrides, setOverrides] = useState<Record<string, CurrencySetting>>({});
  const [editPool, setEditPool] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    api.get<{ items: Currency[] }>("/currencies").then((d) => setCurrencies(d.items)).catch(() => {});
    api.get<{ items: CurrencySetting[] }>("/currency-settings").then((d) => {
      const m: Record<string, CurrencySetting> = {};
      for (const cs of d.items) m[cs.currency] = cs;
      setOverrides(m);
    }).catch(() => {});
  }, []);
  useEffect(load, [load]);

  const save = async (symbol: string, enabled: boolean) => {
    setError(null);
    try {
      await api.put(`/currency-settings/${symbol}`, { pool_url: editPool[symbol] ?? "", enabled });
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e));
    }
  };

  return (
    <div>
      {error ? <div className="error-banner">{error}</div> : null}
      <div className="grid cols-2">
        {currencies.map((c) => {
          const ov = overrides[c.symbol];
          return (
            <div className="card" key={c.symbol}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                <strong>
                  {c.symbol} <span className="hint">{c.name}</span>
                </strong>
                <span className="chip">{c.engine_strategy}</span>
              </div>
              <p className="hint" style={{ fontSize: "var(--fs-1)" }}>
                {c.algorithm} · {c.hardware_class}
                {c.min_vram_mb ? ` · ${t("settings.currencies.min_vram")} ${c.min_vram_mb} MB` : ""}
                {c.engine_strategy === "deferred" ? ` · ${t("settings.currencies.deferred")}` : ""}
              </p>
              <label className="field">
                <span className="field-label">{t("settings.currencies.pool_override")}</span>
                <input
                  type="text"
                  className="mono"
                  placeholder={c.pools?.[0]?.url ?? "stratum+ssl://…"}
                  value={editPool[c.symbol] ?? ov?.pool_url ?? ""}
                  onChange={(e) => setEditPool({ ...editPool, [c.symbol]: e.target.value })}
                />
              </label>
              <div style={{ display: "flex", gap: "var(--sp-3)", alignItems: "center" }}>
                <label style={{ fontSize: "var(--fs-2)" }}>
                  <input
                    type="checkbox"
                    checked={ov?.enabled ?? true}
                    onChange={(e) => save(c.symbol, e.target.checked)}
                  />{" "}
                  {t("settings.currencies.toggle")}
                </label>
                <button className="small" onClick={() => save(c.symbol, ov?.enabled ?? true)}>
                  {t("common.save")}
                </button>
                <a href={c.homepage} target="_blank" rel="noreferrer" style={{ marginLeft: "auto" }}>
                  {t("settings.currencies.homepage")} ↗
                </a>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
