import { Link } from 'react-router-dom'

export function NotFoundPage() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-3 px-6 text-center">
      <p className="text-5xl font-black text-accent">404</p>
      <p className="text-lg font-bold text-text">ページが見つかりません</p>
      <p className="text-sm text-text-dim">URLが間違っているか、ページが削除された可能性があります。</p>
      <Link
        to="/"
        className="mt-4 rounded-full bg-accent px-6 py-2.5 text-sm font-bold text-[#04121a]"
      >
        ホームに戻る
      </Link>
    </div>
  )
}
