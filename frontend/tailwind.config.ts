import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        bg: "#0b0d12",
        surface: "#151922",
        border: "rgba(255,255,255,0.08)",
        accent: "#e8734a",
        success: "#3fb984",
        danger: "#e05263",
      },
      fontFamily: {
        sans: ["InterVariable", "Inter", "system-ui", "sans-serif"],
      },
      borderRadius: {
        card: "12px",
      },
      transitionTimingFunction: {
        game: "cubic-bezier(0.2, 0.8, 0.2, 1)",
      },
    },
  },
  plugins: [],
} satisfies Config;
