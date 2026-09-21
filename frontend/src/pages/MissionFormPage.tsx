import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ApiError } from '../api/client'
import { missionApi } from '../api/endpoints'
import { DIFFICULTIES, DIFFICULTY_XP, type Difficulty, type Weekday } from '../api/types'
import { ErrorBanner } from '../components/ErrorBanner'
import { LoadingSpinner } from '../components/LoadingSpinner'
import { WeekdaySelector } from '../components/WeekdaySelector'

export function MissionFormPage() {
  const { id } = useParams<{ id: string }>()
  const isEdit = Boolean(id)
  const navigate = useNavigate()

  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [difficulty, setDifficulty] = useState<Difficulty>('NORMAL')
  const [days, setDays] = useState<Weekday[]>([])
  const [active, setActive] = useState(true)

  const [loading, setLoading] = useState(isEdit)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return
    let cancelled = false
    missionApi
      .list()
      .then((templates) => {
        if (cancelled) return
        const t = templates.find((x) => x.id === id)
        if (!t) {
          setError('ミッションが見つかりませんでした')
          return
        }
        setTitle(t.title)
        setDescription(t.description)
        setDifficulty(t.difficulty)
        setDays(t.days)
        setActive(t.active)
      })
      .catch((err) => setError(err instanceof ApiError ? err.message : '読み込みに失敗しました'))
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [id])

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (days.length === 0) {
      setError('実行する曜日を1つ以上選択してください')
      return
    }
    setSubmitting(true)
    setError(null)
    try {
      const input = { title, description, difficulty, days, active }
      if (id) {
        await missionApi.update(id, input)
      } else {
        await missionApi.create(input)
      }
      navigate('/missions', { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '保存に失敗しました')
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) return <LoadingSpinner />

  return (
    <div className="flex flex-col gap-5">
      <h1 className="text-lg font-bold text-text">{isEdit ? 'ミッションを編集' : '新しいミッション'}</h1>

      <form onSubmit={onSubmit} className="flex flex-col gap-5">
        {error && <ErrorBanner message={error} />}

        <label className="flex flex-col gap-1.5 text-sm text-text-dim">
          ミッション名
          <input
            required
            maxLength={100}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="例: 5kmランニング"
            className="rounded-xl border border-border bg-surface px-4 py-3 text-text outline-none focus:border-accent"
          />
        </label>

        <label className="flex flex-col gap-1.5 text-sm text-text-dim">
          説明（任意）
          <textarea
            maxLength={500}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={3}
            className="resize-none rounded-xl border border-border bg-surface px-4 py-3 text-text outline-none focus:border-accent"
          />
        </label>

        <div className="flex flex-col gap-1.5 text-sm text-text-dim">
          難易度
          <div className="grid grid-cols-4 gap-2">
            {DIFFICULTIES.map((d) => (
              <button
                key={d}
                type="button"
                onClick={() => setDifficulty(d)}
                aria-pressed={difficulty === d}
                className={`flex flex-col items-center gap-0.5 rounded-xl border py-2.5 text-xs font-semibold transition-colors ${
                  difficulty === d ? 'border-accent bg-accent/15 text-accent' : 'border-border bg-surface text-text-dim'
                }`}
              >
                <span>{d}</span>
                <span className="text-[10px] font-normal opacity-80">+{DIFFICULTY_XP[d]}XP</span>
              </button>
            ))}
          </div>
        </div>

        <div className="flex flex-col gap-1.5 text-sm text-text-dim">
          実行する曜日
          <WeekdaySelector value={days} onChange={setDays} />
        </div>

        <label className="flex items-center justify-between rounded-xl border border-border bg-surface px-4 py-3 text-sm text-text">
          有効にする
          <input
            type="checkbox"
            checked={active}
            onChange={(e) => setActive(e.target.checked)}
            className="h-5 w-5 accent-cyan-400"
          />
        </label>

        <button
          type="submit"
          disabled={submitting}
          className="rounded-full bg-accent py-3.5 text-sm font-bold text-[#04121a] transition-opacity disabled:opacity-50"
        >
          {submitting ? '保存中...' : '保存する'}
        </button>
      </form>
    </div>
  )
}
