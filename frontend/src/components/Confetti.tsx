import { useEffect, useRef } from "react";

const COLOURS = ["#e043f5", "#7c5cff", "#38d9f5", "#fbbf24", "#34d399", "#ffffff"];

interface Particle {
  x: number;
  y: number;
  vx: number;
  vy: number;
  rotation: number;
  vr: number;
  size: number;
  colour: string;
}

/** A restrained 2s canvas confetti burst; a no-op under prefers-reduced-motion (§14.2, §14.3). */
export default function Confetti() {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    canvas.width = window.innerWidth;
    canvas.height = window.innerHeight;

    const particles: Particle[] = Array.from({ length: 80 }, () => ({
      x: Math.random() * canvas.width,
      y: -20 - Math.random() * canvas.height * 0.3,
      vx: (Math.random() - 0.5) * 2,
      vy: 2 + Math.random() * 3,
      rotation: Math.random() * 360,
      vr: (Math.random() - 0.5) * 10,
      size: 6 + Math.random() * 6,
      colour: COLOURS[Math.floor(Math.random() * COLOURS.length)],
    }));

    const start = performance.now();
    let raf = 0;

    const frame = (now: number) => {
      const elapsed = now - start;
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      for (const p of particles) {
        p.x += p.vx;
        p.y += p.vy;
        p.rotation += p.vr;
        ctx.save();
        ctx.translate(p.x, p.y);
        ctx.rotate((p.rotation * Math.PI) / 180);
        ctx.fillStyle = p.colour;
        ctx.fillRect(-p.size / 2, -p.size / 4, p.size, p.size / 2);
        ctx.restore();
      }
      if (elapsed < 2000) {
        raf = requestAnimationFrame(frame);
      } else {
        ctx.clearRect(0, 0, canvas.width, canvas.height);
      }
    };
    raf = requestAnimationFrame(frame);
    return () => cancelAnimationFrame(raf);
  }, []);

  return <canvas ref={canvasRef} className="fixed inset-0 pointer-events-none z-50" aria-hidden />;
}
