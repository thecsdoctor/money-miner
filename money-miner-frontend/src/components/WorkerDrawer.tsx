import React, { useEffect, useState } from "react";
import { api, type Worker } from "../api/client";
import { useT } from "../i18n";
import { StatusDot } from "./StatCard";

// WorkerDrawer — right-side detail drawer: hardware, caps, recent logs.
export function WorkerDrawer({ workerId, onClose }: { workerId: string; onClose: () => void }) {
  const { t } = useT();
  const [data, setData] = useState<{ worker: Worker; online: boolean; logs: string[] } | null>(null);
  useEffect(() => {
    let live = true;
    api
      .get<{ worker: Worker; online: boolean; logs: string[] }>(`/workers/${workerId}`)
      .then((d) => live && setData(d))
      .catch(() => live && setData(null));
    return () => {
      live = false;
    };
  }, [workerId]);

  const w = data?.worker;
  return (
    <div className="drawer" role="dialog" aria-label={t("worker.drawer.title")}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h2 style={{ margin: 0 }}>{w?.name ?? t("worker.drawer.title")}</h2>
        <button className="small" onClick={onClose}>
          {t("common.close")}
        </button>
      </div>
      {w ? (
        <>
          <p>
            <StatusDot status={data?.online ? "connected" : w.status} /> {w.status} ·{" "}
            <span className="chip">{w.kind}</span>
          </p>
          <table className="data">
            <tbody>
              {w.os ? (
                <tr>
                  <td>os/arch</td>
                  <td className="mono">
                    {w.os}/{w.arch}
                  </td>
                </tr>
              ) : null}
              {w.cpu_model ? (
                <tr>
                  <td>cpu</td>
                  <td className="mono">
                    {w.cpu_model} ({w.cpu_cores})
                  </td>
                </tr>
              ) : null}
              {w.gpu_model ? (
                <tr>
                  <td>gpu</td>
                  <td className="mono">
                    {w.gpu_model} ({w.vram_mb} MB)
                  </td>
                </tr>
              ) : null}
              <tr>
                <td>{t("settings.swarm.col.last_seen")}</td>
                <td className="mono">{w.last_seen ? new Date(w.last_seen).toLocaleString() : t("common.never")}</td>
              </tr>
            </tbody>
          </table>
          <h3>{t("worker.drawer.logs")}</h3>
          {data?.logs && data.logs.length > 0 ? (
            <pre className="mono" style={{ fontSize: "var(--fs-1)", whiteSpace: "pre-wrap" }}>
              {data.logs.join("\n")}
            </pre>
          ) : (
            <p className="hint">{t("worker.drawer.no_logs")}</p>
          )}
        </>
      ) : (
        <p className="spinner">{t("common.loading")}</p>
      )}
    </div>
  );
}
