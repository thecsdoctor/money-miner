// Runtime environment — written by entrypoint.sh into /env.js at container
// start and consumed as window.__ENV__ (never build-time baked, dossier 05).
export interface RuntimeEnv {
  API_URL: string;
  KC_URL: string;
  KC_REALM: string;
  KC_CLIENT_ID: string;
}

declare global {
  interface Window {
    __ENV__?: Partial<RuntimeEnv>;
  }
}

export const env: RuntimeEnv = {
  API_URL: window.__ENV__?.API_URL ?? "/api",
  KC_URL: window.__ENV__?.KC_URL ?? "https://auth.thecsdoctor.com",
  KC_REALM: window.__ENV__?.KC_REALM ?? "money-miner",
  KC_CLIENT_ID: window.__ENV__?.KC_CLIENT_ID ?? "money-miner-app",
};
