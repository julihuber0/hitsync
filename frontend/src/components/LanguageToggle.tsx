import { useTranslation } from "react-i18next";

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
      className={`text-xs font-semibold tracking-wide px-2.5 py-1.5 rounded-full border border-border text-neon/70 hover:text-neon hover:border-accent/50 hover:shadow-neon-sm transition-all duration-150 ${className}`}
    >
      {current === "en" ? "DE" : "EN"}
    </button>
  );
}
