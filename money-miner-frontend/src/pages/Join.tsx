import React, { useEffect, useRef, useState } from "react";
import { env } from "../env";
import type { JoinInfo } from "../api/client";
import { useT } from "../i18n";

// /join — PUBLIC browser-worker page (dossier 02/05/07): explicit consent
// screen before anything starts, code-gated (no code → no work, ever),
// default 30% throttle with a 50% hard cap, giant stop button, auto-pause on
// hidden tab, no cookies/tracking. The page always shows whose swarm it
// contributes to.
//
// Honest v0.1.0 state: the wasm hasher reports available=false (native
// kHeavyHash engine ships in v0.2) — the page renders that state plainly
// instead of pretending to hash. Consent, enrollment, and the worker
// registration are real.

declare global {
  interface Window {
    mmNative?: { available: boolean; reason: string; defaultThrottlePct: number; maxThrottlePct: number };
    mmStart?: (opts?: unknown) => string;
    mmStop?: () => void;
    mmStats?: () => { hashrate: number; running: boolean; stats_available: boolean };
  }
}

export function JoinPage() {
  const { t } = useT();
  const [code, setCode] = useState("");
  const [info, setInfo] = useState<JoinInfo | null>(null);
  const [codeError, setCodeError] = useState(false);
  const [consented, setConsented] = useState(false);
  const [throttle, setThrottle] = useState(30);
  const [mining, setMining] = useState(false);
  const [hashrate, setHashrate] = useState(0);
  const [battery, setBattery] = useState<string | null>(null);
  const [keepBackground, setKeepBackground] = useState(false);
  const [wasmLoaded, setWasmLoaded] = useState(false);
  const [wasmReason, setWasmReason] = useState<string | null>(null);
  const [statusMsg, setStatusMsg] = useState<string | null>(null);
  const statsTimer = useRef<ReturnType<typeof setInterval> | null>(null);

  // pre-fill ?code=
  useEffect(() => {
    const q = new URLSearchParams(window.location.search);
    const c = q.get("code");
    if (c) setCode(c);
  }, []);

  // battery status where available
  useEffect(() => {
    const nav = navigator as Navigator & { getBattery?: () => Promise<{ level: number; charging: boolean }> };
    nav.getBattery?.().then((b) => setBattery(`${Math.round(b.level * 100)}%${b.charging ? " ⚡" : ""}`)).catch(() => {});
  }, []);

  // load the wasm hasher wiring (reports its honest availability itself)
  useEffect(() => {
    const load = async () => {
      try {
        // wasm_exec.js + browserhash.wasm are produced by `make build-wasm`
        // into public/wasm; in v0.1.0 the module loads and reports
        // available=false (see reason) rather than hashing.
        await loadScript("/wasm/wasm_exec.js");
        const GoCtor = (window as unknown as { Go?: new () => { importObject: WebAssembly.Imports; run: (i: WebAssembly.Instance) => Promise<void> } }).Go;
        if (!GoCtor) throw new Error("wasm_exec missing");
        const g = new GoCtor();
        const result = await WebAssembly.instantiateStreaming(
          fetch("/wasm/browserhash.wasm"),
          g.importObject,
        );
        void g.run(result.instance);
        setWasmLoaded(true);
        if (window.mmNative && !window.mmNative.available) {
          setWasmReason(window.mmNative.reason);
        }
      } catch {
        // wasm not present in this deployment — same honest state
        setWasmReason("browser hasher module not loaded (v0.1.0 ships it disabled — see docs/roadmap)");
      }
    };
    void load();
  }, []);

  // auto-pause on hidden tab unless the user opts otherwise
  useEffect(() => {
    const onVis = () => {
      if (document.hidden && mining && !keepBackground) {
        stopMining();
      }
    };
    document.addEventListener("visibilitychange", onVis);
    return () => document.removeEventListener("visibilitychange", onVis);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mining, keepBackground]);

  const checkCode = async () => {
    setCodeError(false);
    setStatusMsg(null);
    try {
      const resp = await fetch(`${env.API_URL}/public/join-info?code=${encodeURIComponent(code)}`);
      if (!resp.ok) throw new Error("invalid");
      const d = (await resp.json()) as JoinInfo;
      setInfo(d);
      if (!d.browser_mining.available) {
        setWasmReason((r) => r ?? d.browser_mining.reason);
      }
    } catch {
      setCodeError(true);
    }
  };

  const startMining = async () => {
    if (!window.mmStart) {
      setStatusMsg(wasmReason ?? "hasher unavailable");
      return;
    }
    const err = window.mmStart({ throttle_pct: throttle });
    if (err) {
      setStatusMsg(err);
      return;
    }
    setMining(true);
    statsTimer.current = setInterval(() => {
      const s = window.mmStats?.();
      if (s) setHashrate(s.hashrate);
    }, 2000);
  };

  const stopMining = () => {
    window.mmStop?.();
    setMining(false);
    setHashrate(0);
    if (statsTimer.current) clearInterval(statsTimer.current);
  };

  const available = wasmLoaded && window.mmNative?.available === true;
  const domain = info?.app_domain ?? window.location.host;

  return (
    <div className="join-page">
      <div className="card join-card">
        <h1 style={{ marginTop: 0 }}>{t("join.title")}</h1>
        <p className="hint">{t("join.powered_by", { domain })}</p>

        {!consented ? (
          <>
            <h2>{t("join.consent.title")}</h2>
            <p>{t("join.consent.body", { domain })}</p>
            <button className="primary big-toggle" onClick={() => setConsented(true)}>
              {t("join.consent.agree")}
            </button>
          </>
        ) : (
          <>
            <label className="field">
              <span className="field-label">{t("join.code.label")}</span>
              <input
                className="mono"
                type="text"
                value={code}
                placeholder={t("join.code.placeholder")}
                onChange={(e) => setCode(e.target.value.toUpperCase())}
                onBlur={checkCode}
                autoComplete="off"
                spellCheck={false}
              />
              {codeError ? <div className="error-text">{t("join.code.invalid")}</div> : null}
            </label>

            <label className="field">
              <span className="field-label">
                {t("join.throttle")}: <span className="mono">{throttle}%</span>
              </span>
              <input
                type="range"
                min={10}
                max={50} /* hard cap per dossier 07 */
                step={5}
                value={throttle}
                onChange={(e) => setThrottle(Number(e.target.value))}
              />
              <div className="hint">{t("join.throttle_hint")}</div>
            </label>

            {!available ? (
              <div className="notice">
                <strong>{t("join.unavailable_title")}</strong>
                <br />
                <span style={{ fontSize: "var(--fs-1)" }}>{wasmReason ?? t("common.loading")}</span>
              </div>
            ) : null}

            {statusMsg ? <div className="notice" style={{ marginTop: "var(--sp-3)" }}>{statusMsg}</div> : null}

            <div style={{ marginTop: "var(--sp-4)" }}>
              {mining ? (
                <button className="danger big-toggle" onClick={stopMining}>
                  ■ {t("join.stop")}
                </button>
              ) : (
                <button className="primary big-toggle" disabled={!available || !info} onClick={startMining}>
                  ▶ {t("join.start")}
                </button>
              )}
            </div>

            <div className="join-stats">
              <div>
                <div className="hint">{t("join.hashrate")}</div>
                <div className="mono" style={{ fontSize: "var(--fs-4)" }}>
                  {hashrate.toFixed(1)} H/s
                </div>
              </div>
              <div>
                <div className="hint">{t("join.battery")}</div>
                <div className="mono" style={{ fontSize: "var(--fs-4)" }}>{battery ?? t("common.na")}</div>
              </div>
              <div>
                <div className="hint">status</div>
                <div className="mono" style={{ fontSize: "var(--fs-4)" }}>
                  {mining ? t("join.status.mining") : t("join.status.idle")}
                </div>
              </div>
            </div>

            <label style={{ display: "block", marginTop: "var(--sp-4)", fontSize: "var(--fs-2)" }}>
              <input type="checkbox" checked={keepBackground} onChange={(e) => setKeepBackground(e.target.checked)} />{" "}
              {t("join.background_toggle")}
            </label>
          </>
        )}
      </div>
    </div>
  );
}

function loadScript(src: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const s = document.createElement("script");
    s.src = src;
    s.onload = () => resolve();
    s.onerror = () => reject(new Error(src));
    document.head.appendChild(s);
  });
}
