import React, { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, ApiError, type Currency, type Wallet } from "../api/client";
import { useT } from "../i18n";

// Create-miner wizard: currency → wallet/pool → allocation → review
// (dossier 05). Deferred currencies (XCH) are visible but not selectable;
// GPU coins display the VRAM gate; ASIC-dominated coins carry the honest note.
export function MinerNewPage() {
  const { t } = useT();
  const nav = useNavigate();
  const [step, setStep] = useState(0);
  const [currencies, setCurrencies] = useState<Currency[]>([]);
  const [wallets, setWallets] = useState<Wallet[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const [symbol, setSymbol] = useState("");
  const [walletId, setWalletId] = useState<string | "">("");
  const [newAddress, setNewAddress] = useState("");
  const [poolUrl, setPoolUrl] = useState("");
  const [name, setName] = useState("");
  const [cpuPct, setCpuPct] = useState(50);
  const [gpuPct, setGpuPct] = useState(0);

  useEffect(() => {
    api.get<{ items: Currency[] }>("/currencies").then((d) => setCurrencies(d.items)).catch((e) => setError(e.message));
    api.get<{ items: Wallet[] }>("/wallets").then((d) => setWallets(d.items)).catch(() => {});
  }, []);

  const currency = useMemo(() => currencies.find((c) => c.symbol === symbol), [currencies, symbol]);
  const currencyWallets = wallets.filter((w) => w.currency === symbol);
  const pools = currency?.pools ?? [];
  const asicNote = symbol === "DASH" || symbol === "ZEC" || symbol === "DGB";

  const create = async () => {
    setBusy(true);
    setError(null);
    try {
      let wallet_id: string | undefined = walletId || undefined;
      if (!wallet_id && newAddress.trim()) {
        const w = await api.post<Wallet>("/wallets", { currency: symbol, address: newAddress.trim() });
        wallet_id = w.id;
      }
      await api.post("/miners", {
        name: name || `${symbol} miner`,
        currency: symbol,
        wallet_id,
        pool_url: poolUrl || undefined,
        cpu_pct: cpuPct,
        gpu_pct: gpuPct,
      });
      nav("/miners");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e));
      setBusy(false);
    }
  };

  const steps = ["wizard.step.currency", "wizard.step.wallet", "wizard.step.allocation", "wizard.step.review"];

  return (
    <div style={{ maxWidth: 720 }}>
      <div className="page-head">
        <h1>{t("wizard.title")}</h1>
      </div>
      <div className="tabs">
        {steps.map((s, i) => (
          <button key={s} className={i === step ? "active" : ""} disabled={i > step} onClick={() => setStep(i)}>
            {t(s)}
          </button>
        ))}
      </div>
      {error ? <div className="error-banner">{error}</div> : null}

      {step === 0 ? (
        <div className="card">
          <p className="hint">{t("wizard.currency.select")}</p>
          <div className="grid cols-2">
            {currencies.map((c) => (
              <button
                key={c.symbol}
                onClick={() => c.enabled && c.engine_strategy !== "deferred" && setSymbol(c.symbol)}
                disabled={!c.enabled || c.engine_strategy === "deferred"}
                style={{
                  textAlign: "left",
                  borderColor: symbol === c.symbol ? "var(--brand)" : undefined,
                  display: "block",
                }}
                title={
                  c.engine_strategy === "deferred"
                    ? c.deferred_note ?? t("wizard.currency.deferred")
                    : undefined
                }
              >
                <strong>{c.symbol}</strong> <span className="hint">{c.name}</span>
                <br />
                <span className="hint">
                  {c.algorithm} · {c.hardware_class}
                  {c.min_vram_mb ? ` · ${t("wizard.currency.min_vram", { vram: c.min_vram_mb })}` : ""}
                  {c.engine_strategy === "deferred" ? ` · ${t("settings.currencies.deferred")}` : ""}
                </span>
              </button>
            ))}
          </div>
          {asicNote ? <div className="notice" style={{ marginTop: "var(--sp-4)" }}>{t("wizard.currency.asic_note")}</div> : null}
        </div>
      ) : null}

      {step === 1 ? (
        <div className="card">
          <label className="field">
            <span className="field-label">{t("wizard.wallet.select")}</span>
            <select value={walletId} onChange={(e) => setWalletId(e.target.value)}>
              <option value="">{t("wizard.wallet.new")}</option>
              {currencyWallets.map((w) => (
                <option key={w.id} value={w.id}>
                  {w.label || w.address.slice(0, 16) + "…"}
                </option>
              ))}
            </select>
          </label>
          {!walletId ? (
            <>
              <div className="notice">{t("wizard.wallet.no_seed_note")}</div>
              <label className="field" style={{ marginTop: "var(--sp-4)" }}>
                <span className="field-label">{t("wizard.wallet.address")}</span>
                <input className="mono" type="text" value={newAddress} onChange={(e) => setNewAddress(e.target.value)} spellCheck={false} />
              </label>
            </>
          ) : null}
          <label className="field">
            <span className="field-label">{t("wizard.pool.select")}</span>
            <select value={poolUrl} onChange={(e) => setPoolUrl(e.target.value)}>
              <option value="">{pools.length > 0 ? `${pools[0].name} (${t("common.enabled")})` : t("wizard.pool.custom")}</option>
              {pools.map((p) => (
                <option key={p.url} value={p.url}>
                  {p.name} — {p.url}
                </option>
              ))}
              <option value="__custom">{t("wizard.pool.custom")}</option>
            </select>
          </label>
          {poolUrl === "__custom" || pools.length === 0 ? (
            <label className="field">
              <span className="field-label">{t("wizard.pool.url_label")}</span>
              <input
                className="mono"
                type="text"
                placeholder="stratum+ssl://pool.example.com:443"
                onChange={(e) => setPoolUrl(e.target.value)}
              />
            </label>
          ) : null}
        </div>
      ) : null}

      {step === 2 ? (
        <div className="card">
          <label className="field">
            <span className="field-label">{t("wizard.allocation.cpu_pct", { pct: cpuPct })}</span>
            <input type="range" min={1} max={100} value={cpuPct} onChange={(e) => setCpuPct(Number(e.target.value))} />
          </label>
          <label className="field">
            <span className="field-label">{t("wizard.allocation.gpu_pct", { pct: gpuPct })}</span>
            <input type="range" min={0} max={100} value={gpuPct} onChange={(e) => setGpuPct(Number(e.target.value))} />
          </label>
          {currency?.min_vram_mb && gpuPct > 0 ? (
            <div className="notice">{t("wizard.currency.min_vram", { vram: currency.min_vram_mb })}</div>
          ) : null}
        </div>
      ) : null}

      {step === 3 ? (
        <div className="card">
          <label className="field">
            <span className="field-label">{t("wizard.name_label")}</span>
            <input type="text" value={name} placeholder={`${symbol} miner`} onChange={(e) => setName(e.target.value)} />
          </label>
          <table className="data">
            <tbody>
              <tr>
                <td>{t("wizard.step.currency")}</td>
                <td>
                  <span className="chip">{symbol}</span> {currency?.name}
                </td>
              </tr>
              <tr>
                <td>{t("miners.detail.pool")}</td>
                <td className="mono">{poolUrl && poolUrl !== "__custom" ? poolUrl : pools[0]?.url ?? poolUrl}</td>
              </tr>
              <tr>
                <td>{t("miners.detail.engine")}</td>
                <td className="mono">{currency?.engine_strategy === "native-go" ? "adapter" : currency?.engine_strategy}</td>
              </tr>
              <tr>
                <td>{t("miners.allocation.cpu")} / {t("miners.allocation.gpu")}</td>
                <td className="mono">
                  {cpuPct}% / {gpuPct}%
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      ) : null}

      <div style={{ display: "flex", gap: "var(--sp-2)", marginTop: "var(--sp-4)", justifyContent: "space-between" }}>
        <button disabled={step === 0} onClick={() => setStep(step - 1)}>
          {t("wizard.back")}
        </button>
        {step < 3 ? (
          <button className="primary" disabled={step === 0 && !symbol} onClick={() => setStep(step + 1)}>
            {t("wizard.next")}
          </button>
        ) : (
          <button className="primary" disabled={busy} onClick={create}>
            {t("wizard.create")}
          </button>
        )}
      </div>
    </div>
  );
}
