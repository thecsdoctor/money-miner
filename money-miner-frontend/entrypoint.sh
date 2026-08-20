#!/bin/sh
# entrypoint.sh — render runtime env into env.js (window.__ENV__) then start
# nginx. Runtime config, never build-time baked (dossier 05).
set -eu

WWW=/usr/share/nginx/html
cat > "$WWW/env.js" <<EOF
// generated at container start by entrypoint.sh — do not edit
window.__ENV__ = {
  API_URL: "${MM_API_URL:-/api}",
  KC_URL: "${MM_KC_URL:-https://auth.thecsdoctor.com}",
  KC_REALM: "${MM_KC_REALM:-money-miner}",
  KC_CLIENT_ID: "${MM_KC_CLIENT_ID:-money-miner-app}"
};
EOF

exec "$@"
