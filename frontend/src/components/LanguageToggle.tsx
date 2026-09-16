import { useTranslation } from "react-i18next";
import { Globe } from "lucide-react";

export default function LanguageToggle({ className = "" }: { className?: string }) {
  const { i18n } = useTranslation();
  const current = i18n.language.startsWith("de") ? "de" : "en";

  const toggle = () => {
    const next = current === "en" ? "de" : "en";
    void i18n.changeLanguage(next);
  };

  return (
    <button
      type="button"
      onClick={toggle}
      aria-label="Toggle language"
      className={`inline-flex h-9 items-center gap-1.5 rounded-lg border border-white/10 bg-white/[0.04] px-2.5 text-xs font-semibold tracking-wide text-fg/70 transition-all duration-150 hover:border-white/20 hover:bg-white/[0.08] hover:text-fg ${className}`}
    >
      <Globe size={14} className="text-fg/50" />
      {current === "en" ? "DE" : "EN"}
    </button>
  );
}
