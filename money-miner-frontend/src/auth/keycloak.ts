// Keycloak OAuth2 authorization-code flow with PKCE S256, hand-rolled on
// WebCrypto (dossier 05/07: tokens in memory with refresh rotation; no
// localStorage tokens; no auth library).
//
// In-memory tokens mean a full page reload loses the session and silently
// re-redirects through Keycloak — the realm SSO session makes that instant.
import { env } from "../env";

interface TokenSet {
  access_token: string;
  refresh_token: string;
  expires_in: number;
}

// In-memory session state (module scope — never persisted).
let tokens: TokenSet | null = null;
let expiresAt = 0;
let refreshTimer: ReturnType<typeof setTimeout> | null = null;
let onSessionLost: (() => void) | null = null;

export function setOnSessionLost(fn: () => void) {
  onSessionLost = fn;
}

function base64url(buf: ArrayBuffer | Uint8Array): string {
  const bytes = buf instanceof Uint8Array ? buf : new Uint8Array(buf);
  let s = "";
  for (const b of bytes) s += String.fromCharCode(b);
  return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function randomString(len: number): string {
  const bytes = new Uint8Array(len);
  crypto.getRandomValues(bytes);
  return base64url(bytes).slice(0, len);
}

function endpoints() {
  const base = `${env.KC_URL}/realms/${env.KC_REALM}/protocol/openid-connect`;
  return { auth: `${base}/auth`, token: `${base}/token`, logout: `${base}/logout` };
}

export function isLoggedIn(): boolean {
  return tokens !== null && Date.now() < expiresAt;
}

export function getAccessToken(): string | null {
  return isLoggedIn() ? tokens!.access_token : null;
}

// beginLogin redirects to the realm's login page (auth-code + PKCE S256).
export async function beginLogin(): Promise<void> {
  const verifier = randomString(64);
  const challenge = base64url(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier)));
  const state = randomString(32);
  // Verifier/state are not tokens — sessionStorage is acceptable for these
  // (tokens themselves never touch storage).
  sessionStorage.setItem("mm.pkce.verifier", verifier);
  sessionStorage.setItem("mm.pkce.state", state);
  const redirectUri = `${window.location.origin}/auth/callback`;
  const q = new URLSearchParams({
    client_id: env.KC_CLIENT_ID,
    redirect_uri: redirectUri,
    response_type: "code",
    scope: "openid profile email",
    code_challenge: challenge,
    code_challenge_method: "S256",
    state,
  });
  window.location.assign(`${endpoints().auth}?${q}`);
}

// handleCallback completes the flow at /auth/callback. Returns error text.
export async function handleCallback(search: string): Promise<string | null> {
  const q = new URLSearchParams(search);
  const code = q.get("code");
  const state = q.get("state");
  if (!code || !state) return "missing code/state";
  const expectState = sessionStorage.getItem("mm.pkce.state");
  const verifier = sessionStorage.getItem("mm.pkce.verifier");
  sessionStorage.removeItem("mm.pkce.state");
  sessionStorage.removeItem("mm.pkce.verifier");
  if (!verifier || state !== expectState) return "state mismatch";
  const body = new URLSearchParams({
    grant_type: "authorization_code",
    client_id: env.KC_CLIENT_ID,
    redirect_uri: `${window.location.origin}/auth/callback`,
    code,
    code_verifier: verifier,
  });
  const resp = await fetch(endpoints().token, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body,
  });
  if (!resp.ok) return `token exchange failed (${resp.status})`;
  const ts = (await resp.json()) as TokenSet;
  setTokens(ts);
  return null;
}

function setTokens(ts: TokenSet) {
  tokens = ts;
  expiresAt = Date.now() + ts.expires_in * 1000;
  scheduleRefresh();
}

function scheduleRefresh() {
  if (refreshTimer) clearTimeout(refreshTimer);
  if (!tokens) return;
  // refresh 30 s before expiry (refresh rotation)
  const delay = Math.max(5_000, expiresAt - Date.now() - 30_000);
  refreshTimer = setTimeout(refreshNow, delay);
}

async function refreshNow(): Promise<void> {
  if (!tokens) return;
  try {
    const body = new URLSearchParams({
      grant_type: "refresh_token",
      client_id: env.KC_CLIENT_ID,
      refresh_token: tokens.refresh_token,
    });
    const resp = await fetch(endpoints().token, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body,
    });
    if (!resp.ok) throw new Error(`refresh ${resp.status}`);
    setTokens((await resp.json()) as TokenSet);
  } catch {
    tokens = null;
    onSessionLost?.();
  }
}

// ensureFresh is called by the API client before each request.
export async function ensureFresh(): Promise<void> {
  if (tokens && Date.now() > expiresAt - 15_000) {
    await refreshNow();
  }
}

export function logout(): void {
  if (refreshTimer) clearTimeout(refreshTimer);
  tokens = null;
  const q = new URLSearchParams({
    client_id: env.KC_CLIENT_ID,
    post_logout_redirect_uri: window.location.origin + "/login",
  });
  window.location.assign(`${endpoints().logout}?${q}`);
}
