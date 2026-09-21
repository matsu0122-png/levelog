const DISMISS_KEY = 'levelog:philosophy-dismissed'

import { useState } from 'react'

export function PhilosophyBanner() {
  const [dismissed, setDismissed] = useState(() => {
    try {
      return localStorage.getItem(DISMISS_KEY) === '1'
    } catch {
      return false
    }
  })

  if (dismissed) return null

  const dismiss = () => {
    setDismissed(true)
    try {
      localStorage.setItem(DISMISS_KEY, '1')
    } catch {
      // localStorage unavailable (private browsing etc.) — dismissal just won't persist
    }
  }

  return (
    <div className="flex items-start gap-3 rounded-xl border border-accent/20 bg-accent/5 px-4 py-3 text-sm text-text-dim">
      <p className="flex-1 leading-relaxed">
        このアプリは、あなた自身の成長を記録するためのものです。誰かと競う必要はありません。自分に正直な記録を残しましょう。
      </p>
      <button
        type="button"
        onClick={dismiss}
        aria-label="このメッセージを閉じる"
        className="shrink-0 rounded-full p-1 text-text-dim/70 hover:text-text"
      >
        ✕
      </button>
    </div>
  )
}
