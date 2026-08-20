import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Dev server proxies /api to the local backend; production is same-origin
// through the host nginx edge (see deploy + project AGENTS.md).
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
        rewrite: (p) => p.replace(/^\/api/, "/v1"),
      },
    },
  },
  build: {
    outDir: "dist",
    sourcemap: false,
  },
});
