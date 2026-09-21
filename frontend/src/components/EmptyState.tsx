import type { ReactNode } from 'react'

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string
  description?: string
  action?: ReactNode
}) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-2xl border border-border bg-surface/50 px-6 py-12 text-center">
      <p className="text-base font-medium text-text">{title}</p>
      {description && <p className="max-w-xs text-sm text-text-dim">{description}</p>}
      {action}
    </div>
  )
}
