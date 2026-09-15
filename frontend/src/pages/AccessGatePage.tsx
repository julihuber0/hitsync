import { useState } from "react";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import { api } from "../api/client";
import { useAppStore } from "../store/appStore";
import LanguageToggle from "../components/LanguageToggle";

export default function AccessGatePage() {
  const { t } = useTranslation();
  const setAuthenticated = useAppStore((s) => s.setAuthenticated);
  const [code, setCode] = useState("");
  const [error, setError] = useState(false);
  const [shake, setShake] = useState(0);
  const [submitting, setSubmitting] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!code.trim() || submitting) return;
    setSubmitting(true);
    setError(false);
    try {
      await api.access(code.trim());
      const config = await api.config();
      setAuthenticated(config);
    } catch {
      setError(true);
      setShake((s) => s + 1);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="relative min-h-screen flex items-center justify-center overflow-hidden">
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0"
        style={{
          background:
            "radial-gradient(60% 60% at 30% 20%, rgba(255,43,214,0.18), transparent), radial-gradient(50% 50% at 80% 80%, rgba(0,229,255,0.14), transparent)",
        }}
      />
      <div aria-hidden className="neon-grid-floor pointer-events-none absolute inset-x-0 bottom-0 h-1/2 animate-grid-scroll" />
      <LanguageToggle className="absolute top-6 right-6" />

      <motion.form
        onSubmit={submit}
        key={shake}
        initial={{ opacity: 0, y: 12 }}
        animate={error ? { opacity: 1, y: 0, x: [0, -10, 10, -8, 8, -4, 4, 0] } : { opacity: 1, y: 0 }}
        transition={{ duration: 0.4, ease: "easeOut" }}
        className="relative card-surface shadow-neon w-full max-w-sm mx-4 p-8 flex flex-col items-center gap-6"
      >
        <div className="neon-heading text-2xl font-semibold tracking-tight text-accent">{t("app.name")}</div>
        <div className="text-center">
          <h1 className="text-lg font-semibold">{t("gate.title")}</h1>
          <p className="text-sm text-neon/60 mt-1">{t("gate.subtitle")}</p>
        </div>

        <input
          autoFocus
          value={code}
          onChange={(e) => setCode(e.target.value.toUpperCase())}
          placeholder={t("gate.codePlaceholder")}
          className="neon-focus w-full text-center text-xl font-mono tracking-[0.3em] bg-black/30 border border-border rounded-lg py-3 px-4 outline-none focus:border-accent transition-colors"
          maxLength={32}
        />

        {error && <p className="text-danger text-sm -mt-3">{t("gate.error")}</p>}

        <button
          type="submit"
          disabled={submitting}
          className="w-full bg-accent hover:brightness-110 hover:shadow-neon active:scale-[0.97] transition-all duration-200 text-white font-semibold py-3 rounded-lg disabled:opacity-50 disabled:hover:shadow-none"
        >
          {t("gate.submit")}
        </button>
      </motion.form>
    </div>
  );
}
