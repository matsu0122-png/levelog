import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ApiError } from '../api/client'
import { missionApi } from '../api/endpoints'
import { WEEKDAY_LABEL, type MissionTemplate } from '../api/types'
import { DifficultyBadge } from '../components/DifficultyBadge'
import { EmptyState } from '../components/EmptyState'
import { ErrorBanner } from '../components/ErrorBanner'
import { LoadingSpinner } from '../components/LoadingSpinner'

export function MissionsPage() {
  const [templates, setTemplates] = useState<MissionTemplate[] | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      setTemplates(await missionApi.list())
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '読み込みに失敗しました')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const toggleActive = async (t: MissionTemplate) => {
    setBusyId(t.id)
    setError(null)
    try {
      await missionApi.setActive(t.id, !t.active)
      setTemplates((prev) => prev?.map((x) => (x.id === t.id ? { ...x, active: !x.active } : x)) ?? null)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '通信エラーが発生しました')
    } finally {
      setBusyId(null)
    }
  }

  const remove = async (t: MissionTemplate) => {
    if (!window.confirm(`「${t.title}」を削除しますか？過去の記録は保持されます。`)) return
    setBusyId(t.id)
    setError(null)
    try {
      await missionApi.remove(t.id)
      setTemplates((prev) => prev?.filter((x) => x.id !== t.id) ?? null)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '通信エラーが発生しました')
    } finally {
      setBusyId(null)
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-bold text-text">ミッション管理</h1>
        <Link to="/missions/new" className="rounded-full bg-accent px-4 py-2 text-sm font-bold text-[#04121a]">
          + 新規作成
        </Link>
      </div>

      {error && <ErrorBanner message={error} />}

      {loading && <LoadingSpinner />}

      {!loading && templates && templates.length === 0 && (
        <EmptyState
          title="まだミッションがありません"
          description="日々繰り返す行動をミッションとして登録しましょう。"
          action={
            <Link to="/missions/new" className="rounded-full bg-accent px-5 py-2.5 text-sm font-bold text-[#04121a]">
              最初のミッションを作成
            </Link>
          }
        />
      )}

      {!loading &&
        templates &&
        templates.map((t) => (
          <div key={t.id} className={`rounded-2xl border border-border bg-surface p-4 ${!t.active ? 'opacity-60' : ''}`}>
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <p className="truncate font-semibold text-text">{t.title}</p>
                {t.description && <p className="mt-0.5 truncate text-sm text-text-dim">{t.description}</p>}
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <DifficultyBadge difficulty={t.difficulty} />
                <span className="text-sm font-semibold text-accent">+{t.xpReward}</span>
              </div>
            </div>

            <div className="mt-3 flex flex-wrap gap-1.5">
              {t.days.map((d) => (
                <span key={d} className="rounded-full bg-surface-raised px-2 py-0.5 text-xs text-text-dim">
                  {WEEKDAY_LABEL[d]}
                </span>
              ))}
            </div>

            <div className="mt-4 flex items-center gap-2 border-t border-border pt-3">
              <Link
                to={`/missions/${t.id}/edit`}
                className="rounded-full border border-border px-3 py-1.5 text-xs font-semibold text-text-dim hover:border-accent hover:text-text"
              >
                編集
              </Link>
              <button
                type="button"
                disabled={busyId === t.id}
                onClick={() => toggleActive(t)}
                className="rounded-full border border-border px-3 py-1.5 text-xs font-semibold text-text-dim hover:border-accent hover:text-text disabled:opacity-50"
              >
                {t.active ? '無効化' : '有効化'}
              </button>
              <button
                type="button"
                disabled={busyId === t.id}
                onClick={() => remove(t)}
                className="ml-auto rounded-full border border-red-500/30 px-3 py-1.5 text-xs font-semibold text-red-300 hover:bg-red-500/10 disabled:opacity-50"
              >
                削除
              </button>
            </div>
          </div>
        ))}
    </div>
  )
}
