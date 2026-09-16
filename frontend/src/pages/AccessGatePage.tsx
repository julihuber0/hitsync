import { useState } from "react";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import { api } from "../api/client";
import { useAppStore } from "../store/appStore";
import LanguageToggle from "../components/LanguageToggle";
import { Aurora, Logo } from "../components/ui";

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
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden px-4">
      <Aurora />
      <LanguageToggle className="absolute right-5 top-5" />

      <motion.form
        onSubmit={submit}
        key={shake}
        initial={{ opacity: 0, y: 12 }}
        animate={error ? { opacity: 1, y: 0, x: [0, -10, 10, -8, 8, -4, 4, 0] } : { opacity: 1, y: 0 }}
        transition={{ duration: 0.4, ease: "easeOut" }}
        className="surface relative flex w-full max-w-sm flex-col items-center gap-7 p-8 sm:p-10"
      >
        <Logo size="lg" />
        <div className="text-center">
          <h1 className="text-xl font-semibold tracking-tight">{t("gate.title")}</h1>
          <p className="mt-1.5 text-sm text-fg/55">{t("gate.subtitle")}</p>
        </div>

        <div className="flex w-full flex-col gap-2">
          <input
            autoFocus
            value={code}
            onChange={(e) => setCode(e.target.value.toUpperCase())}
            placeholder={t("gate.codePlaceholder")}
            className={`input h-14 text-center font-mono text-lg tracking-[0.3em] placeholder:tracking-[0.2em] ${
              error ? "border-danger/60 focus:border-danger/60 focus:ring-danger/15" : ""
            }`}
            maxLength={32}
          />
          {error && <p className="text-center text-sm text-danger">{t("gate.error")}</p>}
        </div>

        <button type="submit" disabled={submitting} className="btn btn-primary btn-lg w-full">
          {submitting && <span className="spinner border-white/30 border-t-white" />}
          {t("gate.submit")}
        </button>
      </motion.form>
    </div>
  );
}
