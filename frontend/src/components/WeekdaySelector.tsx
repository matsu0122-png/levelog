import { WEEKDAYS, WEEKDAY_LABEL, type Weekday } from '../api/types'

export function WeekdaySelector({ value, onChange }: { value: Weekday[]; onChange: (days: Weekday[]) => void }) {
  const toggle = (day: Weekday) => {
    if (value.includes(day)) {
      onChange(value.filter((d) => d !== day))
    } else {
      onChange([...value, day])
    }
  }

  return (
    <div className="flex gap-2" role="group" aria-label="実行する曜日">
      {WEEKDAYS.map((day) => {
        const active = value.includes(day)
        return (
          <button
            key={day}
            type="button"
            aria-pressed={active}
            onClick={() => toggle(day)}
            className={`flex h-11 w-11 items-center justify-center rounded-full border text-sm font-semibold transition-colors ${
              active
                ? 'border-accent bg-accent/20 text-accent'
                : 'border-border bg-surface text-text-dim hover:border-accent/40'
            }`}
          >
            {WEEKDAY_LABEL[day]}
          </button>
        )
      })}
    </div>
  )
}
