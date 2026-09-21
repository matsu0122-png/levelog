import type { ReactNode } from 'react'
import { BottomNav } from './BottomNav'

export function Layout({ children }: { children: ReactNode }) {
  return (
    <div className="mx-auto min-h-screen w-full max-w-xl pb-24" style={{ paddingTop: 'env(safe-area-inset-top, 0px)' }}>
      <main className="px-4 pb-6 pt-6">{children}</main>
      <BottomNav />
    </div>
  )
}
