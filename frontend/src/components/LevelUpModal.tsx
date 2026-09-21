import { useEffect, useRef } from 'react'

export function LevelUpModal({
  newLevel,
  xpGained,
  onClose,
}: {
  newLevel: number
  xpGained: number
  onClose: () => void
}) {
  const closeRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    closeRef.current?.focus()
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="levelup-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 px-6 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        className="animate-levelup-pop relative w-full max-w-xs overflow-hidden rounded-3xl border border-accent/40 bg-gradient-to-b from-[#0b1330] to-[#050810] px-6 py-10 text-center shadow-[0_0_60px_rgba(34,211,238,0.25)]"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <div className="animate-levelup-ring h-40 w-40 rounded-full border border-accent/40" />
        </div>
        <div className="animate-levelup-glow pointer-events-none absolute left-1/2 top-1/2 h-56 w-56 -translate-x-1/2 -translate-y-1/2 rounded-full bg-accent/25 blur-3xl" />

        <div className="relative">
          <p id="levelup-title" className="text-sm font-bold tracking-[0.4em] text-accent">
            LEVEL UP
          </p>
          <p className="mt-4 text-6xl font-black tracking-tight text-white drop-shadow-[0_0_18px_rgba(34,211,238,0.7)]">
            Lv.{newLevel}
          </p>
          <p className="mt-3 text-sm text-text-dim">獲得経験値</p>
          <p className="text-xl font-bold text-accent-strong">+{xpGained} XP</p>

          <button
            ref={closeRef}
            type="button"
            onClick={onClose}
            className="mt-8 w-full rounded-full border border-accent/50 bg-accent/10 py-3 text-sm font-semibold text-accent transition-colors hover:bg-accent/20"
          >
            続ける
          </button>
        </div>
      </div>
    </div>
  )
}
