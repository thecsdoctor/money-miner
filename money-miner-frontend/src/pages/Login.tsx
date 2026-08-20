import React, { useEffect, useState } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { beginLogin, handleCallback } from "../auth/keycloak";
import { useT } from "../i18n";

// Login — immediately starts the Keycloak redirect (auth-code + PKCE).
export function LoginPage() {
  const { t } = useT();
  useEffect(() => {
    void beginLogin();
  }, []);
  return (
    <div className="join-page">
      <div className="card join-card" style={{ textAlign: "center" }}>
        <h1 className="brand" style={{ color: "var(--brand)" }}>
          money-miner
        </h1>
        <p>{t("auth.logging_in")}</p>
      </div>
    </div>
  );
}

export function AuthCallbackPage() {
  const { t } = useT();
  const nav = useNavigate();
  const loc = useLocation();
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    handleCallback(loc.search).then((err) => {
      if (err) setError(err);
      else nav("/", { replace: true });
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return (
    <div className="join-page">
      <div className="card join-card" style={{ textAlign: "center" }}>
        {error ? (
          <>
            <p className="error-banner">{t("auth.login_failed")}</p>
            <button className="primary" onClick={() => void beginLogin()}>
              {t("common.retry")}
            </button>
          </>
        ) : (
          <p>{t("common.loading")}</p>
        )}
      </div>
    </div>
  );
}
