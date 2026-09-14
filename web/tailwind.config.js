/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        // ── Grafana-style dark palette ───────────────────────────
        bg:          '#0b0f14',                     // page background
        surface:     '#141a20',                     // cards, panels
        elevated:    '#1a2028',                     // hover, dropdowns
        border:      'rgba(255,255,255,0.06)',
        borderHi:    'rgba(255,255,255,0.12)',

        ink:         '#e6edf3',                     // primary text
        muted:       '#9ba7b3',                     // secondary text
        dim:         '#6b7885',                     // labels, disabled

        // status
        ok:          '#22c55e',
        okBg:        'rgba(34,197,94,0.10)',
        warn:        '#facc15',
        warnBg:      'rgba(250,204,21,0.10)',
        danger:      '#f87171',
        dangerBg:    'rgba(248,113,113,0.10)',
        info:        '#38bdf8',
        infoBg:      'rgba(56,189,248,0.10)',

        // brand accent
        accent:      '#22c55e',
        accentHi:    '#16a34a',
        accentBg:    'rgba(34,197,94,0.08)',
      },
      fontFamily: {
        sans: ['Inter', '-apple-system', 'BlinkMacSystemFont', 'sans-serif'],
        mono: ['"JetBrains Mono"', '"SF Mono"', 'Menlo', 'monospace'],
      },
      boxShadow: {
        card: '0 1px 0 rgba(255,255,255,0.02) inset, 0 1px 3px rgba(0,0,0,0.4)',
      },
    },
  },
  plugins: [],
}
