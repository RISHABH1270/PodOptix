type Kind = 'ok' | 'warn' | 'danger' | 'info' | 'muted'

const styles: Record<Kind, string> = {
  ok:     'text-ok bg-okBg border-ok/30',
  warn:   'text-warn bg-warnBg border-warn/30',
  danger: 'text-danger bg-dangerBg border-danger/30',
  info:   'text-info bg-infoBg border-info/30',
  muted:  'text-dim bg-elevated border-border',
}

const dotColors: Record<Kind, string> = {
  ok: 'bg-ok', warn: 'bg-warn', danger: 'bg-danger', info: 'bg-info', muted: 'bg-dim',
}

export function StatusPill({ kind, label }: { kind: Kind; label: string }) {
  return (
    <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-medium border ${styles[kind]}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${dotColors[kind]}`} />
      {label}
    </span>
  )
}
