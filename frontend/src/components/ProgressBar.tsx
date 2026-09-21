import type { Progress } from '../api/types'

export function LevelProgressBar({ progress }: { progress: Progress }) {
  const pct = progress.xpForNextLevel > 0 ? Math.min(100, Math.round((progress.xpIntoLevel / progress.xpForNextLevel) * 100)) : 0

  return (
    <div className="w-full">
      <div className="mb-1.5 flex items-baseline justify-between text-xs text-text-dim">
        <span>
          Lv.{progress.level} 内 {progress.xpIntoLevel} XP
        </span>
        <span>次のレベルまで {progress.xpForNextLevel - progress.xpIntoLevel} XP</span>
      </div>
      <div
        className="h-3 w-full overflow-hidden rounded-full bg-surface-raised"
        role="progressbar"
        aria-valuenow={pct}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label="次のレベルまでの進捗"
      >
        <div
          className="h-full rounded-full bg-gradient-to-r from-accent-strong to-accent shadow-[0_0_12px_rgba(34,211,238,0.6)] transition-[width] duration-500 ease-out"
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  )
}
