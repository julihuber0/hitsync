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
  build: {
    rolldownOptions: {
      output: {
        // Libraries change far less often than app code; separate chunks keep
        // them cached across deploys and every chunk under the 500 kB warning.
        codeSplitting: {
          groups: [
            { name: "react", test: /[\\/]node_modules[\\/](react|react-dom|scheduler|react-router|cookie|set-cookie-parser)[\\/]/, priority: 30 },
            { name: "motion", test: /[\\/]node_modules[\\/](framer-motion|motion-dom|motion-utils)[\\/]/, priority: 20 },
            { name: "vendor", test: /[\\/]node_modules[\\/]/, priority: 10 },
          ],
        },
      },
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
  },
});
