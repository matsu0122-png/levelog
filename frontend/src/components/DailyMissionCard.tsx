import type { DailyMission } from '../api/types'

export function DailyMissionCard({
  mission,
  onToggle,
  toggling,
  readOnly,
}: {
  mission: DailyMission
  onToggle: () => void
  toggling: boolean
  readOnly?: boolean
}) {
  const completed = mission.status === 'COMPLETED'

  return (
    <div
      className={`flex items-center gap-3 rounded-2xl border px-4 py-3.5 transition-colors ${
        completed ? 'border-accent/25 bg-accent/5' : 'border-border bg-surface'
      }`}
    >
      <button
        type="button"
        onClick={onToggle}
        disabled={toggling || readOnly}
        aria-pressed={completed}
        aria-label={completed ? `${mission.title} を未完了に戻す` : `${mission.title} を完了にする`}
        className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-full border-2 transition-colors disabled:opacity-50 ${
          completed
            ? 'border-accent bg-accent text-[#04121a]'
            : 'border-text-dim/50 text-transparent hover:border-accent'
        }`}
      >
        {toggling ? (
          <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-t-transparent" />
        ) : (
          <CheckIcon />
        )}
      </button>

      <div className="min-w-0 flex-1">
        <p className={`truncate text-sm font-medium ${completed ? 'text-text-dim line-through' : 'text-text'}`}>
          {mission.title}
        </p>
      </div>

      <span className={`shrink-0 text-sm font-semibold tabular-nums ${completed ? 'text-accent' : 'text-text-dim'}`}>
        +{mission.xpReward}
      </span>
    </div>
  )
}

function CheckIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={3}>
      <path d="M5 13l4 4 10-10" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}
