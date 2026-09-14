// PodOptix logo — inlined SVG from assets/logo.svg
// Kept in sync manually (small file, changes rarely).
export function Logo({ size = 40, className = '' }: { size?: number; className?: string }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 200 200"
      width={size}
      height={size}
      className={className}
    >
      <defs>
        <linearGradient id="podoptix-arc" x1="0%" y1="100%" x2="100%" y2="0%">
          <stop offset="0%"   stopColor="#F59E0B" />
          <stop offset="100%" stopColor="#FBBF24" />
        </linearGradient>
      </defs>
      {/* transparent — gauge floats on whatever background it sits on */}
      <path d="M 36 140 A 72 72 0 0 1 164 140" fill="none" stroke="rgba(255,255,255,0.10)" strokeWidth="10" strokeLinecap="round" />
      <path d="M 36 140 A 72 72 0 0 1 120 55"  fill="none" stroke="url(#podoptix-arc)" strokeWidth="10" strokeLinecap="round" />
      <line x1="100" y1="132" x2="118" y2="58" stroke="#FBBF24" strokeWidth="4.5" strokeLinecap="round" />
      <circle cx="100" cy="132" r="8"   fill="#FBBF24" />
    </svg>
  )
}
