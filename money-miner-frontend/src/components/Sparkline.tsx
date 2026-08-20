import React from "react";
import type { MetricPoint } from "../api/client";

// Sparkline — hand-rolled SVG, no chart lib (dossier 05). Area + line.
export function Sparkline({ data, width = 220, height = 48 }: { data: MetricPoint[]; width?: number; height?: number }) {
  if (!data || data.length < 2) {
    return <div style={{ height, color: "var(--ink-faint)", fontSize: "var(--fs-1)" }}>—</div>;
  }
  const max = Math.max(...data.map((d) => d.hashrate), 1);
  const stepX = width / (data.length - 1);
  const pts = data.map((d, i) => [i * stepX, height - (d.hashrate / max) * (height - 4) - 2] as const);
  const line = pts.map(([x, y], i) => `${i === 0 ? "M" : "L"}${x.toFixed(1)},${y.toFixed(1)}`).join(" ");
  const area = `${line} L${width},${height} L0,${height} Z`;
  return (
    <svg className="sparkline" width={width} height={height} viewBox={`0 0 ${width} ${height}`} role="img" aria-hidden="true">
      <path d={area} fill="var(--brand)" opacity={0.14} />
      <path d={line} fill="none" stroke="var(--brand)" strokeWidth={1.5} />
    </svg>
  );
}
