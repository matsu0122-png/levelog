import { useCallback, useEffect, useState } from 'react'
import { ApiError } from '../api/client'
import { dailyMissionApi, missionApi } from '../api/endpoints'
import type { DailyMission, TodayResponse } from '../api/types'
import { DailyMissionCard } from '../components/DailyMissionCard'
import { EmptyState } from '../components/EmptyState'
import { ErrorBanner } from '../components/ErrorBanner'
import { LevelUpModal } from '../components/LevelUpModal'
import { LoadingSpinner } from '../components/LoadingSpinner'
import { PhilosophyBanner } from '../components/PhilosophyBanner'
import { LevelProgressBar } from '../components/ProgressBar'
import { Link } from 'react-router-dom'

export function HomePage() {
  const [data, setData] = useState<TodayResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [togglingId, setTogglingId] = useState<string | null>(null)
  const [levelUp, setLevelUp] = useState<{ newLevel: number; xpGained: number } | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const today = await missionApi.today()
      setData(today)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '読み込みに失敗しました')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const replaceMission = (updated: DailyMission) => {
    setData((prev) => {
      if (!prev) return prev
      const missions = prev.missions.map((m) => (m.id === updated.id ? updated : m))
      const completedCount = missions.filter((m) => m.status === 'COMPLETED').length
      return { ...prev, missions, completedCount }
    })
  }

  const toggle = async (mission: DailyMission) => {
    if (togglingId) return
    setTogglingId(mission.id)
    setError(null)
    try {
      if (mission.status === 'PENDING') {
        const res = await dailyMissionApi.complete(mission.id)
        replaceMission(res.mission)
        setData((prev) => (prev ? { ...prev, progress: res.progress } : prev))
        if (res.leveledUp) {
          setLevelUp({ newLevel: res.progress.level, xpGained: res.xpGained })
        }
      } else {
        const res = await dailyMissionApi.uncomplete(mission.id)
        replaceMission(res.mission)
        setData((prev) => (prev ? { ...prev, progress: res.progress } : prev))
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '通信エラーが発生しました')
    } finally {
      setTogglingId(null)
    }
  }

  if (loading) return <LoadingSpinner label="今日のミッションを読み込み中..." />

  if (error && !data) {
    return (
      <div className="flex flex-col gap-4">
        <ErrorBanner message={error} />
        <button
          type="button"
          onClick={load}
          className="rounded-full border border-border py-2.5 text-sm text-text-dim hover:border-accent"
        >
          再読み込み
        </button>
      </div>
    )
  }

  if (!data) return null

  return (
    <div className="flex flex-col gap-6">
      <PhilosophyBanner />

      <section className="rounded-3xl border border-border bg-surface p-5">
        <div className="flex items-end justify-between">
          <div>
            <p className="text-xs text-text-dim">現在レベル</p>
            <p className="text-4xl font-black text-text">
              Lv.<span className="text-accent">{data.progress.level}</span>
            </p>
          </div>
          <div className="text-right">
            <p className="text-xs text-text-dim">合計XP</p>
            <p className="text-xl font-bold text-text">{data.progress.totalXp}</p>
          </div>
        </div>
        <div className="mt-4">
          <LevelProgressBar progress={data.progress} />
        </div>
      </section>

      <section className="flex items-center justify-between rounded-2xl border border-border bg-surface/60 px-5 py-3">
        <span className="text-sm text-text-dim">今日の達成</span>
        <span className="text-lg font-bold text-text">
          {data.completedCount} <span className="text-sm font-normal text-text-dim">/ {data.totalCount}</span>
        </span>
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="text-sm font-semibold text-text-dim">今日のミッション</h2>

        {error && <ErrorBanner message={error} />}

        {data.missions.length === 0 ? (
          <EmptyState
            title="今日のミッションはまだありません"
            description="ミッション管理から、繰り返しミッションを作成しましょう。"
            action={
              <Link to="/missions/new" className="rounded-full bg-accent px-5 py-2.5 text-sm font-bold text-[#04121a]">
                ミッションを作成
              </Link>
            }
          />
        ) : (
          data.missions.map((m) => (
            <DailyMissionCard key={m.id} mission={m} onToggle={() => toggle(m)} toggling={togglingId === m.id} />
          ))
        )}
      </section>

      {levelUp && (
        <LevelUpModal newLevel={levelUp.newLevel} xpGained={levelUp.xpGained} onClose={() => setLevelUp(null)} />
      )}
    </div>
  )
}
