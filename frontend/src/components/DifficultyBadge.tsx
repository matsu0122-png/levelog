import type { Difficulty } from '../api/types'

const STYLES: Record<Difficulty, string> = {
  EASY: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30',
  NORMAL: 'bg-sky-500/15 text-sky-300 border-sky-500/30',
  HARD: 'bg-amber-500/15 text-amber-300 border-amber-500/30',
  EXTREME: 'bg-rose-500/15 text-rose-300 border-rose-500/30',
}

export function DifficultyBadge({ difficulty }: { difficulty: Difficulty }) {
  return (
    <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-semibold tracking-wide ${STYLES[difficulty]}`}>
      {difficulty}
    </span>
  )
}
