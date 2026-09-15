import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          admin: ["./src/pages/AdminPage.tsx"],
        },
      },
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
  },
});
