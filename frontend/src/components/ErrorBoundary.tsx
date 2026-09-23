import { Component, type ErrorInfo, type ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
}

// Catches render/lifecycle errors anywhere below it in the tree so a bug in
// one screen shows a recoverable fallback instead of a blank white page.
// Does not catch errors from async code (fetch, event handlers) — those are
// handled per-page via ApiError + ErrorBanner instead.
export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Unhandled UI error', error, info.componentStack)
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex min-h-screen flex-col items-center justify-center gap-4 px-6 text-center">
          <p className="text-lg font-bold text-text">予期しないエラーが発生しました</p>
          <p className="text-sm text-text-dim">お手数ですが、ページを再読み込みしてください。</p>
          <button
            type="button"
            onClick={() => window.location.reload()}
            className="rounded-full bg-accent px-6 py-2.5 text-sm font-bold text-[#04121a]"
          >
            再読み込み
          </button>
        </div>
      )
    }
    return this.props.children
  }
}
