import React from "react";
import { fmtHashrate, type MinerRun } from "../api/client";
import { useT } from "../i18n";

// RunHistoryList — one row per start→stop session (full history retained).
export function RunHistoryList({ runs }: { runs: MinerRun[] }) {
  const { t } = useT();
  if (!runs || runs.length === 0) {
    return <p className="hint">{t("miners.detail.no_runs")}</p>;
  }
  return (
    <table className="data">
      <thead>
        <tr>
          <th>{t("miners.detail.started")}</th>
          <th>{t("miners.detail.stopped")}</th>
          <th>{t("miners.detail.reason")}</th>
          <th>{t("miners.table.hashrate")} ⌀</th>
          <th>{t("miners.table.shares")}</th>
        </tr>
      </thead>
      <tbody>
        {runs.map((r) => (
          <tr key={r.id}>
            <td className="mono">{new Date(r.started_at).toLocaleString()}</td>
            <td className="mono">{r.stopped_at ? new Date(r.stopped_at).toLocaleString() : t("common.na")}</td>
            <td>{r.stop_reason ?? t("common.na")}</td>
            <td className="mono">{fmtHashrate(r.avg_hashrate || 0)}</td>
            <td className="mono">
              {r.shares_accepted}/{r.shares_rejected}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
