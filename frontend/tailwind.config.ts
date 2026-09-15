import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        // 80s neon / synthwave palette.
        bg: "#0a0118",
        surface: "#170b2e",
        border: "rgba(255,190,247,0.18)",
        // Primary interactive colour (hot magenta).
        accent: "#ff2bd6",
        // Secondary neon highlight (electric cyan) for duotone accents.
        accent2: "#00e5ff",
        success: "#39ff88",
        danger: "#ff3864",
        // Primary text colour per the 80s neon theme.
        neon: "#ffbef7",
      },
      fontFamily: {
        sans: ["InterVariable", "Inter", "system-ui", "sans-serif"],
        display: ["'Orbitron Variable'", "InterVariable", "Inter", "system-ui", "sans-serif"],
      },
      borderRadius: {
        card: "12px",
      },
      transitionTimingFunction: {
        game: "cubic-bezier(0.2, 0.8, 0.2, 1)",
      },
      boxShadow: {
        neon: "0 0 10px rgba(255,43,214,0.55), 0 0 28px rgba(255,43,214,0.25)",
        "neon-sm": "0 0 6px rgba(255,43,214,0.5)",
        "neon-cyan": "0 0 10px rgba(0,229,255,0.5), 0 0 24px rgba(0,229,255,0.2)",
        "neon-inset": "inset 0 1px 0 rgba(255,255,255,0.06), 0 0 24px rgba(255,43,214,0.08)",
      },
      dropShadow: {
        neon: "0 0 6px rgba(255,43,214,0.65)",
        "neon-cyan": "0 0 6px rgba(0,229,255,0.65)",
      },
      keyframes: {
        "fade-in": {
          from: { opacity: "0", transform: "translateY(6px)" },
          to: { opacity: "1", transform: "translateY(0)" },
        },
        "glow-pulse": {
          "0%, 100%": { opacity: "0.55" },
          "50%": { opacity: "1" },
        },
        "grid-scroll": {
          from: { backgroundPosition: "0 0" },
          to: { backgroundPosition: "0 56px" },
        },
      },
      animation: {
        "fade-in": "fade-in 0.35s cubic-bezier(0.2, 0.8, 0.2, 1) both",
        "glow-pulse": "glow-pulse 2.4s ease-in-out infinite",
        "grid-scroll": "grid-scroll 6s linear infinite",
      },
    },
  },
  plugins: [],
} satisfies Config;
