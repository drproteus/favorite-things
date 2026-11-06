import { Globe, Lock } from 'lucide-react'
import type { Visibility } from '../types'

export default function VisibilityToggle({
  value,
  onChange,
}: {
  value: Visibility
  onChange: (v: Visibility) => void
}) {
  const options: { value: Visibility; icon: typeof Lock; text: string }[] = [
    { value: 'private', icon: Lock, text: 'Private' },
    { value: 'public', icon: Globe, text: 'Public' },
  ]
  return (
    <div role="radiogroup" aria-label="Visibility" className="grid grid-cols-2 gap-1 rounded-lg bg-stone-100 p-1 dark:bg-stone-800">
      {options.map((o) => {
        const active = value === o.value
        return (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onChange(o.value)}
            className={`flex cursor-pointer items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition ${
              active
                ? 'bg-white text-indigo-700 shadow-sm dark:bg-stone-900 dark:text-indigo-300'
                : 'text-stone-500 hover:text-stone-800 dark:text-stone-400 dark:hover:text-stone-200'
            }`}
          >
            <o.icon size={15} />
            {o.text}
          </button>
        )
      })}
    </div>
  )
}
