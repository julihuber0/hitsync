import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        // Near-black canvas with a faint violet cast.
        bg: "#07070c",
        surface: "#101018",
        elevated: "#171722",
        border: "rgba(255,255,255,0.08)",
        // Foreground text; muted tones use opacity (text-fg/60).
        fg: "#f4f4f7",
        // Brand gradient runs accent -> accent-end.
        accent: "#e043f5",
        "accent-end": "#7c5cff",
        accent2: "#38d9f5",
        gold: "#fbbf24",
        success: "#34d399",
        danger: "#fb7185",
      },
      fontFamily: {
        sans: ["InterVariable", "Inter", "system-ui", "sans-serif"],
      },
      borderRadius: {
        card: "20px",
      },
      transitionTimingFunction: {
        game: "cubic-bezier(0.2, 0.8, 0.2, 1)",
      },
      boxShadow: {
        glow: "0 10px 30px -10px rgba(224,67,245,0.55)",
        "glow-sm": "0 4px 16px -4px rgba(224,67,245,0.5)",
        "glow-cyan": "0 8px 24px -8px rgba(56,217,245,0.45)",
        card: "inset 0 1px 0 rgba(255,255,255,0.05), 0 24px 48px -24px rgba(0,0,0,0.7)",
      },
      keyframes: {
        "fade-in": {
          from: { opacity: "0", transform: "translateY(8px)" },
          to: { opacity: "1", transform: "translateY(0)" },
        },
        "glow-pulse": {
          "0%, 100%": { opacity: "0.5" },
          "50%": { opacity: "1" },
        },
        equalizer: {
          "0%, 100%": { transform: "scaleY(0.3)" },
          "50%": { transform: "scaleY(1)" },
        },
        aurora: {
          "0%, 100%": { transform: "translate3d(0,0,0) scale(1)" },
          "50%": { transform: "translate3d(2%,3%,0) scale(1.08)" },
        },
      },
      animation: {
        "fade-in": "fade-in 0.4s cubic-bezier(0.2, 0.8, 0.2, 1) both",
        "glow-pulse": "glow-pulse 2.4s ease-in-out infinite",
        equalizer: "equalizer 1.1s ease-in-out infinite",
        aurora: "aurora 18s ease-in-out infinite",
      },
    },
  },
  plugins: [],
} satisfies Config;
