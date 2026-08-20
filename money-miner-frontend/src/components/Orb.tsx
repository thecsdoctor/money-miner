import React, { useEffect, useMemo, useRef, useState } from "react";
import type { OrbWorker } from "../api/client";
import { fmtHashrate } from "../api/client";
import { useT } from "../i18n";

// Orb — the dashboard swarm visualization (dossier 05). Canvas-2D, one
// requestAnimationFrame loop, devicePixelRatio scaling, <50 nodes. Central
// sphere breathes (4 s), amber while mining; workers orbit on Kepler-ish
// rings (outer slower), size ∝ log10(hashrate), color by status; browser
// workers get an --info ring. prefers-reduced-motion → static layout.
// A11y: role="img" + aria-label + a visually-hidden workers table.

interface Props {
  workers: OrbWorker[];
  onSelect?: (workerId: string) => void;
}

interface NodeState {
  id: string;
  ring: number;
  angle: number;
  joinTs: number;
  leaveTs: number; // 0 = active
}

function cssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || "#888";
}

export function Orb({ workers, onSelect }: Props) {
  const { t } = useT();
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const wrapRef = useRef<HTMLDivElement>(null);
  const nodesRef = useRef<Map<string, NodeState>>(new Map());
  const workersRef = useRef<OrbWorker[]>(workers);
  const hoverRef = useRef<string | null>(null);
  const [tooltip, setTooltip] = useState<{ x: number; y: number; text: string } | null>(null);
  const [pulse, setPulse] = useState(0);

  workersRef.current = workers;

  // metrics-tick pulse: connecting lines breathe when fresh data arrives.
  useEffect(() => {
    setPulse((p) => p + 1);
  }, [workers]);

  const online = workers.filter((w) => w.status === "mining" || w.status === "connected").length;
  const totalHash = workers.reduce((a, w) => a + (w.hashrate || 0), 0);
  const reduced = useMemo(
    () => window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false,
    [],
  );

  useEffect(() => {
    const canvas = canvasRef.current;
    const wrap = wrapRef.current;
    if (!canvas || !wrap) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    let raf = 0;
    let width = 0;
    let height = 0;

    const resize = () => {
      const rect = wrap.getBoundingClientRect();
      const dpr = window.devicePixelRatio || 1;
      width = rect.width;
      height = Math.min(420, Math.max(280, rect.width * 0.66));
      canvas.width = width * dpr;
      canvas.height = height * dpr;
      canvas.style.height = `${height}px`;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };
    resize();
    const ro = new ResizeObserver(resize);
    ro.observe(wrap);

    const syncNodes = () => {
      const seen = new Set<string>();
      const list = workersRef.current;
      list.forEach((w, i) => {
        seen.add(w.id);
        const nodes = nodesRef.current;
        if (!nodes.has(w.id)) {
          nodes.set(w.id, {
            id: w.id,
            ring: i % 3,
            angle: (i / Math.max(1, list.length)) * Math.PI * 2,
            joinTs: performance.now(),
            leaveTs: 0,
          });
        }
      });
      for (const [id, n] of nodesRef.current) {
        if (!seen.has(id) && n.leaveTs === 0) n.leaveTs = performance.now();
      }
    };

    const draw = (now: number) => {
      syncNodes();
      ctx.clearRect(0, 0, width, height);
      const cx = width / 2;
      const cy = height / 2;
      const baseR = Math.min(width, height) * 0.34;

      const mining = workersRef.current.some((w) => w.status === "mining");
      const brandColor = cssVar("--brand");
      const faintColor = cssVar("--ink-faint");
      const okColor = cssVar("--ok");
      const warnColor = cssVar("--warn");
      const infoColor = cssVar("--info");
      const lineColor = cssVar("--line");

      // central orb with 4 s breathing glow
      const breathe = reduced ? 0.5 : (Math.sin((now / 4000) * Math.PI * 2) + 1) / 2;
      const orbR = baseR * 0.42 * (1 + (reduced ? 0 : 0.03 * breathe));
      const orbColor = mining ? brandColor : faintColor;
      const grad = ctx.createRadialGradient(cx - orbR * 0.3, cy - orbR * 0.3, orbR * 0.1, cx, cy, orbR);
      grad.addColorStop(0, orbColor);
      grad.addColorStop(1, "transparent");
      ctx.globalAlpha = mining ? 0.85 + 0.15 * breathe : 0.45;
      ctx.fillStyle = grad;
      ctx.beginPath();
      ctx.arc(cx, cy, orbR, 0, Math.PI * 2);
      ctx.fill();
      ctx.globalAlpha = 1;

      // orbit rings
      ctx.strokeStyle = lineColor;
      ctx.globalAlpha = 0.35;
      for (let ring = 0; ring < 3; ring++) {
        const rx = baseR * (0.72 + ring * 0.42);
        const ry = rx * 0.62;
        ctx.beginPath();
        ctx.ellipse(cx, cy, rx, ry, 0, 0, Math.PI * 2);
        ctx.stroke();
      }
      ctx.globalAlpha = 1;

      // worker nodes
      const remove: string[] = [];
      for (const n of nodesRef.current.values()) {
        const w = workersRef.current.find((x) => x.id === n.id);
        if (!reduced) {
          // Kepler-ish: outer rings slower
          const speed = 0.00016 / (1 + n.ring * 0.7);
          n.angle += speed * 16;
        }
        // join spiral-in / leave fade + collapse
        let scale = 1;
        let alpha = 1;
        if (now - n.joinTs < 900 && !reduced) {
          const p = (now - n.joinTs) / 900;
          scale = 1.6 - 0.6 * p;
          alpha = p;
        }
        if (n.leaveTs > 0) {
          const p = (now - n.leaveTs) / 700;
          if (p >= 1) {
            remove.push(n.id);
            continue;
          }
          alpha = 1 - p;
          scale = 1 - 0.5 * p;
        }
        const rx = baseR * (0.72 + n.ring * 0.42) * scale;
        const ry = rx * 0.62;
        const x = cx + Math.cos(n.angle) * rx;
        const y = cy + Math.sin(n.angle) * ry;

        const status = w?.status ?? "offline";
        const color =
          status === "mining" ? okColor : status === "connected" || status === "idle" ? warnColor : faintColor;
        const hr = w?.hashrate ?? 0;
        const nodeR = (4 + Math.log10(Math.max(1, hr)) * 2.2) * (reduced ? 1 : 1);

        // connecting line, opacity pulses with the metrics tick
        const pulsePhase = reduced ? 0.3 : (Math.sin(now / 500 + n.angle * 3) + 1) / 2;
        ctx.globalAlpha = alpha * (status === "mining" ? 0.25 + 0.5 * pulsePhase : 0.12);
        ctx.strokeStyle = color;
        ctx.beginPath();
        ctx.moveTo(cx, cy);
        ctx.lineTo(x, y);
        ctx.stroke();

        // node
        ctx.globalAlpha = alpha;
        ctx.fillStyle = color;
        ctx.beginPath();
        ctx.arc(x, y, Math.max(3, nodeR), 0, Math.PI * 2);
        ctx.fill();
        if (w?.kind === "browser") {
          ctx.strokeStyle = infoColor;
          ctx.lineWidth = 1.5;
          ctx.beginPath();
          ctx.arc(x, y, Math.max(3, nodeR) + 3, 0, Math.PI * 2);
          ctx.stroke();
          ctx.lineWidth = 1;
        }
        if (hoverRef.current === n.id) {
          ctx.strokeStyle = brandColor;
          ctx.beginPath();
          ctx.arc(x, y, Math.max(3, nodeR) + 5, 0, Math.PI * 2);
          ctx.stroke();
        }
        n.ringX = x;
        n.ringY = y;
        n.nodeR = Math.max(3, nodeR) + 6;
      }
      remove.forEach((id) => nodesRef.current.delete(id));
      ctx.globalAlpha = 1;

      raf = requestAnimationFrame(draw);
    };
    raf = requestAnimationFrame(draw);

    const hitTest = (ev: MouseEvent): string | null => {
      const rect = canvas.getBoundingClientRect();
      const x = ev.clientX - rect.left;
      const y = ev.clientY - rect.top;
      for (const n of nodesRef.current.values()) {
        const nr = (n as NodeState & { nodeR?: number }).nodeR ?? 10;
        const nx = (n as NodeState & { ringX?: number }).ringX ?? 0;
        const ny = (n as NodeState & { ringY?: number }).ringY ?? 0;
        if ((x - nx) ** 2 + (y - ny) ** 2 <= nr * nr) return n.id;
      }
      return null;
    };

    const onMove = (ev: MouseEvent) => {
      const id = hitTest(ev);
      hoverRef.current = id;
      canvas.style.cursor = id ? "pointer" : "default";
      if (id) {
        const w = workersRef.current.find((x) => x.id === id);
        const rect = canvas.getBoundingClientRect();
        setTooltip({
          x: ev.clientX - rect.left + 12,
          y: ev.clientY - rect.top + 12,
          text: `${w?.name ?? id} · ${w?.kind ?? ""} · ${fmtHashrate(w?.hashrate ?? 0)}${w?.currency ? " · " + w.currency : ""}`,
        });
      } else {
        setTooltip(null);
      }
    };
    const onClick = (ev: MouseEvent) => {
      const id = hitTest(ev);
      if (id && onSelect) onSelect(id);
    };
    canvas.addEventListener("mousemove", onMove);
    canvas.addEventListener("click", onClick);
    canvas.addEventListener("mouseleave", () => setTooltip(null));

    return () => {
      cancelAnimationFrame(raf);
      ro.disconnect();
      canvas.removeEventListener("mousemove", onMove);
      canvas.removeEventListener("click", onClick);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [reduced, pulse > 0]);

  return (
    <div className="orb-wrap" ref={wrapRef}>
      <canvas
        ref={canvasRef}
        role="img"
        aria-label={t("dashboard.orb_aria", {
          online: online,
          total: workers.length,
          hashrate: fmtHashrate(totalHash),
        })}
      />
      {tooltip ? (
        <div className="orb-tooltip" style={{ left: tooltip.x, top: tooltip.y, display: "block" }}>
          {tooltip.text}
        </div>
      ) : null}
      {/* screen-reader-exposed worker list (color is never the only signal) */}
      <table className="visually-hidden">
        <caption>workers</caption>
        <thead>
          <tr>
            <th>name</th>
            <th>kind</th>
            <th>status</th>
            <th>hashrate</th>
          </tr>
        </thead>
        <tbody>
          {workers.map((w) => (
            <tr key={w.id}>
              <td>{w.name}</td>
              <td>{w.kind}</td>
              <td>{w.status}</td>
              <td>{fmtHashrate(w.hashrate)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// NodeState gets mutable render-time coordinates for hit testing.
interface NodeState {
  ringX?: number;
  ringY?: number;
  nodeR?: number;
}
