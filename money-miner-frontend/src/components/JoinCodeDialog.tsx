import React, { useEffect, useState } from "react";
import type { NewJoinCode } from "../api/client";
import { useT } from "../i18n";

// JoinCodeDialog — code in large mono, copy button, 15-min countdown ring,
// enroll one-liner (dossier 05). The plaintext code is shown exactly once.
export function JoinCodeDialog({ code, onClose }: { code: NewJoinCode; onClose: () => void }) {
  const { t } = useT();
  const [copied, setCopied] = useState(false);
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const iv = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(iv);
  }, []);
  const expires = new Date(code.expires_at).getTime();
  const total = 15 * 60 * 1000;
  const left = Math.max(0, expires - now);
  const frac = left / total;
  const minutes = Math.ceil(left / 60000);

  // countdown ring
  const r = 26;
  const circ = 2 * Math.PI * r;

  const copy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard denied — the text is selectable */
    }
  };

  return (
    <div className="dialog-backdrop" onClick={onClose}>
      <div className="dialog" onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <h2 style={{ marginTop: 0 }}>{t("settings.swarm.generate")}</h2>
        <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-4)" }}>
          <svg width={64} height={64} viewBox="0 0 64 64" aria-hidden="true">
            <circle cx={32} cy={32} r={r} fill="none" stroke="var(--line)" strokeWidth={5} />
            <circle
              cx={32}
              cy={32}
              r={r}
              fill="none"
              stroke={frac > 0.25 ? "var(--brand)" : "var(--err)"}
              strokeWidth={5}
              strokeDasharray={circ}
              strokeDashoffset={circ * (1 - frac)}
              strokeLinecap="round"
              transform="rotate(-90 32 32)"
            />
            <text x={32} y={37} textAnchor="middle" fill="var(--ink)" fontSize={13} fontFamily="var(--mono)">
              {left > 0 ? `${minutes}m` : "✕"}
            </text>
          </svg>
          <p className="hint" style={{ margin: 0 }}>
            {left > 0
              ? t("settings.swarm.code_expires", { minutes })
              : t("settings.swarm.countdown_expired")}
          </p>
        </div>
        <div className="code-display" aria-label="join code">
          {code.code}
        </div>
        <p className="hint">{t("settings.swarm.enroll_hint")}</p>
        <div className="one-liner">{code.enroll_command}</div>
        <div style={{ display: "flex", gap: "var(--sp-2)", marginTop: "var(--sp-4)", justifyContent: "flex-end" }}>
          <button onClick={() => copy(code.enroll_command)}>{copied ? t("common.copied") : t("common.copy")}</button>
          <button className="primary" onClick={onClose}>
            {t("common.close")}
          </button>
        </div>
      </div>
    </div>
  );
}
