export function LoadingSpinner({ label = '読み込み中...' }: { label?: string }) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-16 text-text-dim" role="status" aria-live="polite">
      <div className="h-8 w-8 animate-spin rounded-full border-2 border-border border-t-accent" />
      <span className="text-sm">{label}</span>
    </div>
  )
}
