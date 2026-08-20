import React, { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, type Miner } from "../api/client";
import { MinersTable } from "../components/MinersTable";
import { useT } from "../i18n";

// Miners — the table page (create wizard is /miners/new, detail /miners/:id).
export function MinersPage() {
  const { t } = useT();
  const nav = useNavigate();
  const [miners, setMiners] = useState<Miner[]>([]);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    api
      .get<{ items: Miner[] }>("/miners")
      .then((d) => setMiners(d.items))
      .catch((e) => setError(e.message));
  }, []);

  useEffect(load, [load]);

  return (
    <div>
      <div className="page-head">
        <h1>{t("miners.title")}</h1>
        <button className="primary" onClick={() => nav("/miners/new")}>
          {t("miners.new")}
        </button>
      </div>
      {error ? <div className="error-banner">{error}</div> : null}
      <div className="card">
        <MinersTable miners={miners} onChanged={load} onEmptyAction={() => nav("/miners/new")} />
      </div>
    </div>
  );
}
