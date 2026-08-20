import React, { useCallback, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  api,
  fmtHashrate,
  type MetricPoint,
  type MinerDetail,
  type MinerRun,
} from "../api/client";
import { RunHistoryList } from "../components/RunHistoryList";
import { SimBadge, StatusDot } from "../components/StatCard";
import { Sparkline } from "../components/Sparkline";
import { useT } from "../i18n";

// Miner detail — live stats, metrics chart, allocation sliders, run history.
export function MinerDetailPage() {
  const { t } = useT();
  const { id } = useParams<{ id: string }>();
  const nav = useNavigate();
  const [miner, setMiner] = useState<MinerDetail | null>(null);
  const [series, setSeries] = useState<MetricPoint[]>([]);
  const [runs, setRuns] = useState<MinerRun[]>([]);
  const [cpu, setCpu] = useState(50);
  const [gpu, setGpu] = useState(0);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    if (!id) return;
    api
      .get<MinerDetail>(`/miners/${id}`)
      .then((m) => {
        setMiner(m);
        setCpu(m.cpu_pct);
        setGpu(m.gpu_pct);
      })
      .catch((e) => setError(e.message));
    api
      .get<{ runs: MinerRun[]; series: MetricPoint[] }>(`/miners/${id}/history`)
      .then((d) => {
        setRuns(d.runs);
        setSeries(d.series);
      })
      .catch(() => {});
    api
      .get<{ items: MetricPoint[] }>(`/miners/${id}/metrics?limit=120`)
      .then(() => {})
      .catch(() => {});
  }, [id]);

  useEffect(() => {
    load();
    const iv = setInterval(load, 10_000);
    return () => clearInterval(iv);
  }, [load]);

  if (error) {
    return (
      <div>
        <div className="error-banner">{error}</div>
        <button onClick={() => nav("/miners")}>{t("wizard.back")}</button>
      </div>
    );
  }
  if (!miner) return <p className="spinner">{t("common.loading")}</p>;

  const act = async (action: "start" | "stop") => {
    try {
      await api.post(`/miners/${miner.id}/${action}`);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  };
  const applyAllocation = async () => {
    await api.post(`/miners/${miner.id}/allocation`, { cpu_pct: cpu, gpu_pct: gpu });
    load();
  };
  const remove = async () => {
    if (!window.confirm(t("miners.delete_confirm"))) return;
    await api.del(`/miners/${miner.id}`);
    nav("/miners");
  };

  return (
    <div>
      <div className="page-head">
        <div>
          <h1>
            {miner.name} {miner.simulated ? <SimBadge /> : null}
          </h1>
          <div className="sub">
            <span className="chip">{miner.currency}</span>{" "}
            <StatusDot status={miner.status} />
            {t(`miners.status.${miner.status}`)} · <span className="mono">{fmtHashrate(miner.hashrate)}</span>
          </div>
        </div>
        <div style={{ display: "flex", gap: "var(--sp-2)" }}>
          {miner.status === "running" || miner.status === "queued" ? (
            <button onClick={() => act("stop")}>{t("common.stop")}</button>
          ) : (
            <button className="primary" onClick={() => act("start")}>
              {t("common.start")}
            </button>
          )}
          <button className="danger" onClick={remove}>
            {t("common.delete")}
          </button>
        </div>
      </div>

      <div className="grid cols-2">
        <div className="card">
          <h3>{t("miners.detail.history")}</h3>
          <Sparkline data={series} width={520} height={120} />
        </div>
        <div className="card">
          <h3>{t("miners.allocation.cpu")} / {t("miners.allocation.gpu")}</h3>
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
          <button className="small primary" onClick={applyAllocation}>
            {t("miners.allocation.apply")}
          </button>
        </div>
      </div>

      <div className="card" style={{ marginTop: "var(--sp-4)" }}>
        <table className="data">
          <tbody>
            <tr>
              <td>{t("miners.detail.pool")}</td>
              <td className="mono">{miner.pool_url}</td>
            </tr>
            <tr>
              <td>{t("miners.detail.engine")}</td>
              <td className="mono">{miner.engine}</td>
            </tr>
            <tr>
              <td>{t("miners.detail.workers")}</td>
              <td>
                {miner.workers && miner.workers.length > 0
                  ? miner.workers.map((w) => (
                      <span key={w.id} className="chip" style={{ marginRight: 4 }}>
                        {w.name}
                      </span>
                    ))
                  : t("common.na")}
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <h2>{t("miners.detail.runs")}</h2>
      <div className="card">
        <RunHistoryList runs={runs} />
      </div>
    </div>
  );
}
