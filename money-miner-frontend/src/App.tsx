import React from "react";
import { NavLink, Outlet, Route, Routes, useLocation } from "react-router-dom";
import { beginLogin, isLoggedIn, logout, setOnSessionLost } from "./auth/keycloak";
import { LangSwitcher, ThemeToggle } from "./components/Toggles";
import { useT } from "./i18n";
import { DashboardPage } from "./pages/Dashboard";
import { JoinPage } from "./pages/Join";
import { AuthCallbackPage, LoginPage } from "./pages/Login";
import { MinerDetailPage } from "./pages/MinerDetail";
import { MinerNewPage } from "./pages/MinerNew";
import { MinersPage } from "./pages/Miners";
import { SettingsPage } from "./pages/Settings";
import { useNavigate } from "react-router-dom";

// Route guard: everything except /login, /auth/callback and /join requires
// an in-memory session (dossier 05).
function RequireAuth() {
  const loc = useLocation();
  if (!isLoggedIn()) {
    void beginLogin();
    return <p className="spinner">…</p>;
  }
  return <Outlet key={loc.pathname} />;
}

function Shell() {
  const { t } = useT();
  const nav = useNavigate();
  React.useEffect(() => {
    setOnSessionLost(() => nav("/login"));
  }, [nav]);
  return (
    <div className="app-shell">
      <nav className="sidebar">
        <div className="brand">money-miner</div>
        <NavLink to="/" end className={({ isActive }) => "nav-link" + (isActive ? " active" : "")}>
          {t("nav.dashboard")}
        </NavLink>
        <NavLink to="/miners" className={({ isActive }) => "nav-link" + (isActive ? " active" : "")}>
          {t("nav.miners")}
        </NavLink>
        <NavLink to="/settings" className={({ isActive }) => "nav-link" + (isActive ? " active" : "")}>
          {t("nav.settings")}
        </NavLink>
        <a className="nav-link" href="/join" target="_blank" rel="noreferrer">
          {t("nav.join_public")} ↗
        </a>
        <div className="sidebar-foot">
          <ThemeToggle />
          <LangSwitcher />
          <button className="small" onClick={logout} title={t("nav.logout")}>
            ⏻
          </button>
        </div>
      </nav>
      <main className="main">
        <Outlet />
      </main>
    </div>
  );
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/auth/callback" element={<AuthCallbackPage />} />
      <Route path="/join" element={<JoinPage />} />
      <Route element={<RequireAuth />}>
        <Route element={<Shell />}>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/miners" element={<MinersPage />} />
          <Route path="/miners/new" element={<MinerNewPage />} />
          <Route path="/miners/:id" element={<MinerDetailPage />} />
          <Route path="/settings" element={<SettingsPage />} />
        </Route>
      </Route>
    </Routes>
  );
}
