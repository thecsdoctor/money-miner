import React from "react";
import { useT } from "../i18n";

export function StatCard({ label, value, delta }: { label: string; value: string; delta?: string }) {
  return (
    <div className="card stat-card">
      <div className="stat-label">{label}</div>
      <div className="stat-value">{value}</div>
      {delta ? <div className="stat-delta">{delta}</div> : null}
    </div>
  );
}

export function SimBadge() {
  const { t } = useT();
  return (
    <span className="sim-badge" title={t("sim.tooltip")}>
      {t("sim.badge")}
    </span>
  );
}

export function EmptyState({ title, actionLabel, onAction }: { title: string; actionLabel?: string; onAction?: () => void }) {
  return (
    <div className="empty-state">
      <div className="empty-title">{title}</div>
      {actionLabel && onAction ? (
        <button className="primary" onClick={onAction}>
          {actionLabel}
        </button>
      ) : null}
    </div>
  );
}

export function StatusDot({ status }: { status: string }) {
  const cls =
    status === "running" || status === "mining" || status === "connected"
      ? "ok"
      : status === "queued" || status === "assigned" || status === "idle" || status === "enrolled"
        ? "warn"
        : status === "error" || status === "revoked"
          ? "err"
          : "idle";
  return <span className={`dot ${cls}`} aria-hidden="true" />;
}
