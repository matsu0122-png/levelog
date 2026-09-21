import { useEffect, useState } from 'react'
import { ApiError } from '../api/client'
import { missionApi } from '../api/endpoints'
import type { HistoryDay } from '../api/types'
import { EmptyState } from '../components/EmptyState'
import { ErrorBanner } from '../components/ErrorBanner'
import { LoadingSpinner } from '../components/LoadingSpinner'

function formatDateLabel(dateStr: string) {
  const d = new Date(`${dateStr}T00:00:00`)
  const weekday = ['日', '月', '火', '水', '木', '金', '土'][d.getDay()]
  return `${d.getMonth() + 1}/${d.getDate()} (${weekday})`
}

export function HistoryPage() {
  const [days, setDays] = useState<HistoryDay[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    missionApi
      .history(7)
      .then((res) => {
        if (!cancelled) setDays([...res].reverse())
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof ApiError ? err.message : '読み込みに失敗しました')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  if (loading) return <LoadingSpinner />
  if (error) return <ErrorBanner message={error} />
  if (!days) return null

  const hasAny = days.some((d) => d.totalCount > 0)

  return (
    <div className="flex flex-col gap-5">
      <h1 className="text-lg font-bold text-text">過去7日間の履歴</h1>

      {!hasAny && (
        <EmptyState title="まだ履歴がありません" description="ミッションを完了すると、ここに記録が表示されます。" />
      )}

      {days.map((day) => (
        <section key={day.date} className="rounded-2xl border border-border bg-surface p-4">
          <div className="mb-2 flex items-center justify-between">
            <p className="text-sm font-semibold text-text">{formatDateLabel(day.date)}</p>
            <div className="flex items-center gap-3 text-xs text-text-dim">
              <span>
                {day.completedCount}/{day.totalCount} 達成
              </span>
              {day.xpEarned > 0 && <span className="font-semibold text-accent">+{day.xpEarned} XP</span>}
            </div>
          </div>

          {day.missions.length === 0 ? (
            <p className="text-xs text-text-dim">記録なし</p>
          ) : (
            <ul className="flex flex-col gap-1.5">
              {day.missions.map((m) => (
                <li key={m.id} className="flex items-center justify-between text-sm">
                  <span className={m.status === 'COMPLETED' ? 'text-text' : 'text-text-dim'}>{m.title}</span>
                  <span className={m.status === 'COMPLETED' ? 'text-accent' : 'text-text-dim'}>
                    {m.status === 'COMPLETED' ? '完了' : '未完了'}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </section>
      ))}
    </div>
  )
}
