import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ApiError } from '../api/client'
import { ErrorBanner } from '../components/ErrorBanner'
import { useAuth } from '../context/AuthContext'

export function SettingsPage() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)
  const [loggingOut, setLoggingOut] = useState(false)

  const onLogout = async () => {
    setLoggingOut(true)
    setError(null)
    try {
      await logout()
      navigate('/login', { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '通信エラーが発生しました')
    } finally {
      setLoggingOut(false)
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <h1 className="text-lg font-bold text-text">設定</h1>

      {error && <ErrorBanner message={error} />}

      <section className="rounded-2xl border border-border bg-surface p-4">
        <dl className="flex flex-col gap-3 text-sm">
          <div className="flex items-center justify-between">
            <dt className="text-text-dim">メールアドレス</dt>
            <dd className="font-medium text-text">{user?.email}</dd>
          </div>
          <div className="flex items-center justify-between">
            <dt className="text-text-dim">タイムゾーン</dt>
            <dd className="font-medium text-text">{user?.timezone}</dd>
          </div>
        </dl>
      </section>

      <section className="rounded-2xl border border-accent/20 bg-accent/5 p-4 text-sm leading-relaxed text-text-dim">
        levelogは、他人と競うランキングを持ちません。記録されるのはあなた自身の継続だけです。自己申告を、自分自身への信頼として積み重ねましょう。
      </section>

      <button
        type="button"
        onClick={onLogout}
        disabled={loggingOut}
        className="rounded-full border border-border py-3 text-sm font-semibold text-text-dim hover:border-red-500/40 hover:text-red-300 disabled:opacity-50"
      >
        {loggingOut ? 'ログアウト中...' : 'ログアウト'}
      </button>
    </div>
  )
}
