import { useEffect, useState } from "react";
import QRCode from "qrcode";

export default function QrCode({ value, size = 160 }: { value: string; size?: number }) {
  const [dataUrl, setDataUrl] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void QRCode.toDataURL(value, { width: size, margin: 1, color: { dark: "#0b0d12", light: "#ffffff" } }).then((url) => {
      if (!cancelled) setDataUrl(url);
    });
    return () => {
      cancelled = true;
    };
  }, [value, size]);

  if (!dataUrl) return <div style={{ width: size, height: size }} className="bg-white/5 rounded-lg animate-pulse" />;
  return <img src={dataUrl} alt="Invite QR code" width={size} height={size} className="rounded-lg" />;
}
