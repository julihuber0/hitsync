import { useEffect, useState } from "react";
import QRCode from "qrcode";

export default function QrCode({ value, size = 160 }: { value: string; size?: number }) {
  const [dataUrl, setDataUrl] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void QRCode.toDataURL(value, { width: size, margin: 1, color: { dark: "#07070c", light: "#ffffff" } }).then((url) => {
      if (!cancelled) setDataUrl(url);
    });
    return () => {
      cancelled = true;
    };
  }, [value, size]);

  if (!dataUrl) return <div style={{ width: size + 16, height: size + 16 }} className="shrink-0 animate-pulse rounded-2xl bg-white/5" />;
  return (
    <img
      src={dataUrl}
      alt="Invite QR code"
      width={size}
      height={size}
      className="shrink-0 rounded-2xl bg-white p-2 shadow-glow-sm"
      style={{ width: size + 16, height: size + 16 }}
    />
  );
}
