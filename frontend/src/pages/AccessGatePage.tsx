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
    <div className="relative min-h-screen flex items-center justify-center overflow-hidden bg-bg">
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0"
        style={{
          background:
            "radial-gradient(60% 60% at 30% 20%, rgba(232,115,74,0.12), transparent), radial-gradient(50% 50% at 80% 80%, rgba(63,185,132,0.10), transparent)",
        }}
      />
      <LanguageToggle className="absolute top-6 right-6" />

      <motion.form
        onSubmit={submit}
        key={shake}
        animate={error ? { x: [0, -10, 10, -8, 8, -4, 4, 0] } : {}}
        transition={{ duration: 0.4, ease: "easeOut" }}
        className="relative card-surface w-full max-w-sm mx-4 p-8 flex flex-col items-center gap-6"
      >
        <div className="text-2xl font-semibold tracking-tight text-accent">{t("app.name")}</div>
        <div className="text-center">
          <h1 className="text-lg font-semibold">{t("gate.title")}</h1>
          <p className="text-sm text-white/60 mt-1">{t("gate.subtitle")}</p>
        </div>

        <input
          autoFocus
          value={code}
          onChange={(e) => setCode(e.target.value.toUpperCase())}
          placeholder={t("gate.codePlaceholder")}
          className="w-full text-center text-xl font-mono tracking-[0.3em] bg-black/30 border border-border rounded-lg py-3 px-4 outline-none focus:border-accent transition-colors"
          maxLength={32}
        />

        {error && <p className="text-danger text-sm -mt-3">{t("gate.error")}</p>}

        <button
          type="submit"
          disabled={submitting}
          className="w-full bg-accent hover:brightness-110 transition-[filter] duration-150 text-white font-semibold py-3 rounded-lg disabled:opacity-50"
        >
          {t("gate.submit")}
        </button>
      </motion.form>
    </div>
  );
}
