import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ApiError } from '../api/client'
import { ErrorBanner } from '../components/ErrorBanner'
import { useAuth } from '../context/AuthContext'

export function LoginPage() {
  const { login, sessionExpired, dismissSessionExpired } = useAuth()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => dismissSessionExpired, [dismissSessionExpired])

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await login(email, password)
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '通信エラーが発生しました')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-6">
      <div className="w-full max-w-sm">
        <h1 className="mb-1 text-center text-2xl font-bold text-text">levelog</h1>
        <p className="mb-8 text-center text-sm text-text-dim">自分に嘘をつかず、成長を記録する</p>

        <form onSubmit={onSubmit} className="flex flex-col gap-4">
          {sessionExpired && !error && (
            <ErrorBanner message="セッションの有効期限が切れました。再度ログインしてください。" />
          )}
          {error && <ErrorBanner message={error} />}

          <label className="flex flex-col gap-1.5 text-sm text-text-dim">
            メールアドレス
            <input
              type="email"
              required
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="rounded-xl border border-border bg-surface px-4 py-3 text-text outline-none focus:border-accent"
            />
          </label>

          <label className="flex flex-col gap-1.5 text-sm text-text-dim">
            パスワード
            <input
              type="password"
              required
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="rounded-xl border border-border bg-surface px-4 py-3 text-text outline-none focus:border-accent"
            />
          </label>

          <button
            type="submit"
            disabled={submitting}
            className="mt-2 rounded-full bg-accent py-3.5 text-sm font-bold text-[#04121a] transition-opacity disabled:opacity-50"
          >
            {submitting ? 'ログイン中...' : 'ログイン'}
          </button>
        </form>

        <p className="mt-6 text-center text-sm text-text-dim">
          アカウントをお持ちでないですか？{' '}
          <Link to="/register" className="font-semibold text-accent">
            新規登録
          </Link>
        </p>
      </div>
    </div>
  )
}
