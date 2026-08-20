import React, { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  api,
  fmtHashrate,
  subscribeEvents,
  type Miner,
  type Payout,
  type SwarmStats,
} from "../api/client";
import { Orb } from "../components/Orb";
import { MinersTable } from "../components/MinersTable";
import { StatCard } from "../components/StatCard";
import { WorkerDrawer } from "../components/WorkerDrawer";
import { useT } from "../i18n";

// Dashboard — orb card + swarm totals + per-miner table + recent payouts
// (dossier 05). Live via SSE /v1/events (metrics_tick every 5 s).
export function DashboardPage() {
  const { t } = useT();
  const nav = useNavigate();
  const [stats, setStats] = useState<SwarmStats | null>(null);
  const [miners, setMiners] = useState<Miner[]>([]);
  const [payouts, setPayouts] = useState<Payout[]>([]);
  const [drawerWorker, setDrawerWorker] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    api
      .get<{ items: Miner[] }>("/miners")
      .then((d) => setMiners(d.items))
      .catch((e) => setError(e.message));
    api
      .get<{ items: Payout[] }>("/payouts?limit=5")
      .then((d) => setPayouts(d.items))
      .catch(() => {});
    api
      .get<SwarmStats>("/swarm/stats")
      .then(setStats)
      .catch(() => {});
  }, []);

  useEffect(() => {
    load();
    // SSE: live orb payload + miner status changes. Reconnect on error
    // after a short delay (proxy-friendly, dossier 04).
    let closed = false;
    let close = () => {};
    const connect = () => {
      close = subscribeEvents(
        (ev) => {
          if (ev.type === "metrics_tick") {
            const d = ev.data as SwarmStats;
            setStats((prev) => ({
              workers: d.workers,
              totals: { ...prev?.totals, ...d.totals } as SwarmStats["totals"],
            }));
          } else if (ev.type === "miner_status" || ev.type === "payout_detected") {
            load();
          }
        },
        () => {
          if (!closed) setTimeout(connect, 3000);
        },
      );
    };
    connect();
    return () => {
      closed = true;
      close();
    };
  }, [load]);

  const totals = stats?.totals;
  return (
    <div>
      <div className="page-head">
        <div>
          <h1>{t("dashboard.title")}</h1>
          <div className="sub">{t("dashboard.subtitle")}</div>
        </div>
      </div>
      {error ? <div className="error-banner">{error}</div> : null}

      <div className="grid orb-layout">
        <div className="card">
          <Orb workers={stats?.workers ?? []} onSelect={(id) => setDrawerWorker(id)} />
        </div>
        <div className="grid cols-2">
          <StatCard
            label={t("dashboard.workers_online")}
            value={`${totals?.workers_online ?? 0}/${totals?.workers_total ?? 0}`}
          />
          <StatCard label={t("dashboard.total_hashrate")} value={fmtHashrate(totals?.hashrate ?? 0)} />
          <StatCard label={t("dashboard.shares_24h")} value={String(totals?.shares_24h ?? 0)} />
          <StatCard
            label={t("dashboard.payouts_24h")}
            value={(totals?.payouts_24h ?? 0).toFixed(4)}
          />
        </div>
      </div>

      <h2>{t("dashboard.your_miners")}</h2>
      <div className="card">
        <MinersTable miners={miners} onChanged={load} onEmptyAction={() => nav("/miners/new")} />
      </div>

      <h2>{t("dashboard.recent_payouts")}</h2>
      <div className="card">
        {payouts.length === 0 ? (
          <p className="hint">{t("dashboard.no_payouts")}</p>
        ) : (
          <table className="data">
            <thead>
              <tr>
                <th>{t("payouts.date")}</th>
                <th>{t("payouts.currency")}</th>
                <th>{t("payouts.amount")}</th>
                <th>{t("payouts.tx")}</th>
              </tr>
            </thead>
            <tbody>
              {payouts.map((p) => (
                <tr key={p.id}>
                  <td className="mono">{new Date(p.detected_at).toLocaleString()}</td>
                  <td>
                    <span className="chip">{p.currency}</span>
                  </td>
                  <td className="mono">{p.amount}</td>
                  <td>
                    {p.explorer_url ? (
                      <a href={p.explorer_url} target="_blank" rel="noreferrer">
                        {t("dashboard.view_tx")} ↗
                      </a>
                    ) : (
                      t("payouts.unverified")
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <p className="economics-note">{t("economics.note")}</p>

      {drawerWorker ? <WorkerDrawer workerId={drawerWorker} onClose={() => setDrawerWorker(null)} /> : null}
    </div>
  );
}
