import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// Local dev only (§ docs/local-development.md): the production build is
// static files served by nginx, which proxies nothing itself — Traefik
// routes /api and /ws to the backend by path. In `vite dev`, there is no
// Traefik in front, so we proxy the same paths straight to the backend
// container/process running on :8080.
const BACKEND_URL = process.env.VITE_BACKEND_URL ?? "http://localhost:8080";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      "/api": { target: BACKEND_URL, changeOrigin: true },
      "/healthz": { target: BACKEND_URL, changeOrigin: true },
      "/ws": { target: BACKEND_URL, ws: true, changeOrigin: true },
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
  },
});
