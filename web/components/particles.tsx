'use client';

import { useEffect, useRef } from 'react';

/**
 * 星尘粒子背景。
 * 一个固定定位的 canvas，漂浮若干带柔光的粒子，鼠标靠近时轻微扰动。
 * 尊重 prefers-reduced-motion；移动端降低粒子数以省电。
 */
export function Particles() {
  const ref = useRef<HTMLCanvasElement | null>(null);

  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;

    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const isMobile = window.matchMedia('(max-width: 768px)').matches;
    const count = reduce ? 0 : isMobile ? 26 : 60;

    if (count === 0) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    let w = 0;
    let h = 0;
    let raf = 0;

    // 二次元糖果色调色板
    const palette = ['#ff5d9e', '#8b5cf6', '#22d3ee', '#a78bfa', '#ff9bc9', '#5eead4'];

    type Dot = {
      x: number;
      y: number;
      r: number;
      vx: number;
      vy: number;
      color: string;
      alpha: number;
      tw: number; // 闪烁相位
    };

    let dots: Dot[] = [];
    let mouseX = -9999;
    let mouseY = -9999;

    const resize = () => {
      w = canvas.clientWidth;
      h = canvas.clientHeight;
      canvas.width = Math.round(w * dpr);
      canvas.height = Math.round(h * dpr);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };

    const seed = () => {
      dots = Array.from({ length: count }, () => ({
        x: Math.random() * w,
        y: Math.random() * h,
        r: Math.random() * 2.2 + 0.6,
        vx: (Math.random() - 0.5) * 0.22,
        vy: (Math.random() - 0.5) * 0.22,
        color: palette[Math.floor(Math.random() * palette.length)],
        alpha: Math.random() * 0.55 + 0.25,
        tw: Math.random() * Math.PI * 2,
      }));
    };

    const onMouse = (e: MouseEvent) => {
      mouseX = e.clientX;
      mouseY = e.clientY;
    };
    const onLeave = () => {
      mouseX = -9999;
      mouseY = -9999;
    };

    const tick = (t: number) => {
      ctx.clearRect(0, 0, w, h);

      for (const d of dots) {
        d.x += d.vx;
        d.y += d.vy;

        // 鼠标附近轻微排斥
        const dx = d.x - mouseX;
        const dy = d.y - mouseY;
        const dist2 = dx * dx + dy * dy;
        if (dist2 < 9000) {
          const dist = Math.sqrt(dist2) || 1;
          const f = (1 - dist / 95) * 0.5;
          d.x += (dx / dist) * f;
          d.y += (dy / dist) * f;
        }

        if (d.x < -10) d.x = w + 10;
        if (d.x > w + 10) d.x = -10;
        if (d.y < -10) d.y = h + 10;
        if (d.y > h + 10) d.y = -10;

        const pulse = d.alpha * (0.65 + 0.35 * Math.sin(t * 0.001 + d.tw));

        // 柔光
        const g = ctx.createRadialGradient(d.x, d.y, 0, d.x, d.y, d.r * 5);
        g.addColorStop(0, d.color + 'aa');
        g.addColorStop(1, d.color + '00');
        ctx.fillStyle = g;
        ctx.beginPath();
        ctx.arc(d.x, d.y, d.r * 5, 0, Math.PI * 2);
        ctx.fill();

        // 核心亮点
        ctx.globalAlpha = pulse;
        ctx.fillStyle = d.color;
        ctx.beginPath();
        ctx.arc(d.x, d.y, d.r, 0, Math.PI * 2);
        ctx.fill();
        ctx.globalAlpha = 1;
      }

      raf = requestAnimationFrame(tick);
    };

    resize();
    seed();
    raf = requestAnimationFrame(tick);

    window.addEventListener('resize', resize);
    window.addEventListener('mousemove', onMouse, { passive: true });
    window.addEventListener('mouseleave', onLeave);

    return () => {
      cancelAnimationFrame(raf);
      window.removeEventListener('resize', resize);
      window.removeEventListener('mousemove', onMouse);
      window.removeEventListener('mouseleave', onLeave);
    };
  }, []);

  return <canvas ref={ref} className="fx-canvas" aria-hidden="true" />;
}
