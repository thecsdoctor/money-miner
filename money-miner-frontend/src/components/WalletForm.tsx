import React, { useState } from "react";
import { api, ApiError, type Currency } from "../api/client";
import { useT } from "../i18n";

// WalletForm — per-coin address validation feedback (server-side family
// validation; the no-seed-phrase honesty note is always visible).
export function WalletForm({
  currencies,
  exclude,
  onSaved,
}: {
  currencies: Currency[];
  exclude?: string[]; // currencies that already have a wallet
  onSaved: () => void;
}) {
  const { t } = useT();
  const available = currencies.filter((c) => c.enabled && !(exclude ?? []).includes(c.symbol));
  const [symbol, setSymbol] = useState(available[0]?.symbol ?? "");
  const [address, setAddress] = useState("");
  const [label, setLabel] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      await api.post("/wallets", { currency: symbol, address, label });
      setAddress("");
      setLabel("");
      onSaved();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  if (available.length === 0) return null;

  return (
    <div className="card" style={{ marginTop: "var(--sp-4)" }}>
      <div className="notice" style={{ marginBottom: "var(--sp-4)" }}>
        {t("wizard.wallet.no_seed_note")}
      </div>
      <label className="field">
        <span className="field-label">{t("settings.wallets.col.currency")}</span>
        <select value={symbol} onChange={(e) => setSymbol(e.target.value)}>
          {available.map((c) => (
            <option key={c.symbol} value={c.symbol}>
              {c.symbol} — {c.name}
            </option>
          ))}
        </select>
      </label>
      <label className="field">
        <span className="field-label">{t("wizard.wallet.address")}</span>
        <input
          type="text"
          value={address}
          onChange={(e) => setAddress(e.target.value)}
          className="mono"
          spellCheck={false}
          autoComplete="off"
        />
      </label>
      <label className="field">
        <span className="field-label">{t("wizard.wallet.label")}</span>
        <input type="text" value={label} onChange={(e) => setLabel(e.target.value)} />
      </label>
      {error ? <div className="error-banner">{error}</div> : null}
      <button className="primary" disabled={busy || !address.trim()} onClick={submit}>
        {t("settings.wallets.add")}
      </button>
    </div>
  );
}
