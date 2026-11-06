import type { ReactNode } from 'react'
import { card } from '../ui'

export function Loading() {
  return (
    <div className="flex justify-center py-16">
      <div className="h-8 w-8 animate-spin rounded-full border-4 border-stone-200 border-t-indigo-600 dark:border-stone-700" />
    </div>
  )
}

export function ErrorState({ message }: { message: string }) {
  return (
    <div className={`${card} p-6 text-center text-sm text-rose-600 dark:text-rose-400`}>{message}</div>
  )
}

export function EmptyState({ icon, title, children }: { icon: ReactNode; title: string; children?: ReactNode }) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-2xl border border-dashed border-stone-300 px-6 py-16 text-center dark:border-stone-700">
      <div className="text-stone-400">{icon}</div>
      <h3 className="text-lg font-semibold">{title}</h3>
      {children && <div className="text-sm text-stone-500 dark:text-stone-400">{children}</div>}
    </div>
  )
}
