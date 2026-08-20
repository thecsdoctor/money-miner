import React, { useState } from "react";
import { Link } from "react-router-dom";
import { api, fmtHashrate, type Miner } from "../api/client";
import { useT } from "../i18n";
import { EmptyState, SimBadge, StatusDot } from "./StatCard";

// MinersTable — status dot, currency chip, live hashrate, inline start/stop,
// allocation sliders in the expanded row (dossier 05).
export function MinersTable({
  miners,
  onChanged,
  onEmptyAction,
}: {
  miners: Miner[];
  onChanged: () => void;
  onEmptyAction: () => void;
}) {
  const { t } = useT();
  const [expanded, setExpanded] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const act = async (m: Miner, action: "start" | "stop") => {
    setBusy(m.id);
    setError(null);
    try {
      await api.post(`/miners/${m.id}/${action}`);
      onChanged();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
    }
  };

  if (miners.length === 0) {
    return <EmptyState title={t("empty.miners.title")} actionLabel={t("empty.miners.action")} onAction={onEmptyAction} />;
  }

  return (
    <div>
      {error ? <div className="error-banner">{error}</div> : null}
      <table className="data">
        <thead>
          <tr>
            <th>{t("miners.table.name")}</th>
            <th>{t("miners.table.currency")}</th>
            <th>{t("miners.table.status")}</th>
            <th>{t("miners.table.hashrate")}</th>
            <th>{t("miners.table.shares")}</th>
            <th>{t("miners.table.actions")}</th>
          </tr>
        </thead>
        <tbody>
          {miners.map((m) => (
            <React.Fragment key={m.id}>
              <tr>
                <td>
                  <Link to={`/miners/${m.id}`}>{m.name}</Link>
                  {m.simulated ? <SimBadge /> : null}
                </td>
                <td>
                  <span className="chip">{m.currency}</span>
                </td>
                <td>
                  <StatusDot status={m.status} />
                  {t(`miners.status.${m.status}`)}
                </td>
                <td className="hashrate">{fmtHashrate(m.hashrate)}</td>
                <td className="mono">
                  {m.shares_accepted}/{m.shares_rejected}
                </td>
                <td>
                  {m.status === "running" || m.status === "queued" ? (
                    <button className="small" disabled={busy === m.id} onClick={() => act(m, "stop")}>
                      {t("common.stop")}
                    </button>
                  ) : (
                    <button className="small primary" disabled={busy === m.id} onClick={() => act(m, "start")}>
                      {t("common.start")}
                    </button>
                  )}{" "}
                  <button className="small" onClick={() => setExpanded(expanded === m.id ? null : m.id)}>
                    %
                  </button>
                </td>
              </tr>
              {expanded === m.id ? (
                <tr>
                  <td colSpan={6}>
                    <AllocationRow miner={m} onChanged={onChanged} />
                  </td>
                </tr>
              ) : null}
            </React.Fragment>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function AllocationRow({ miner, onChanged }: { miner: Miner; onChanged: () => void }) {
  const { t } = useT();
  const [cpu, setCpu] = useState(miner.cpu_pct);
  const [gpu, setGpu] = useState(miner.gpu_pct);
  const [saved, setSaved] = useState(false);
  const apply = async () => {
    await api.post(`/miners/${miner.id}/allocation`, { cpu_pct: cpu, gpu_pct: gpu });
    setSaved(true);
    setTimeout(() => setSaved(false), 1500);
    onChanged();
  };
  return (
    <div className="grid cols-2" style={{ padding: "var(--sp-3) 0" }}>
      <label className="field">
        <span className="field-label">
          {t("miners.allocation.cpu")}: <span className="mono">{cpu}%</span>
        </span>
        <input type="range" min={1} max={100} value={cpu} onChange={(e) => setCpu(Number(e.target.value))} />
      </label>
      <label className="field">
        <span className="field-label">
          {t("miners.allocation.gpu")}: <span className="mono">{gpu}%</span>
        </span>
        <input type="range" min={0} max={100} value={gpu} onChange={(e) => setGpu(Number(e.target.value))} />
      </label>
      <div>
        <button className="small primary" onClick={apply}>
          {t("miners.allocation.apply")}
        </button>{" "}
        {saved ? <span className="hint">{t("settings.saved")}</span> : null}
      </div>
    </div>
  );
}
