import { Star } from 'lucide-react'

interface Props {
  value: number
  onChange?: (value: number) => void
  size?: number
  disabled?: boolean
}

export default function StarRating({ value, onChange, size = 18, disabled = false }: Props) {
  const interactive = onChange !== undefined && !disabled
  const rounded = Math.round(value)

  return (
    <div className="inline-flex items-center gap-0.5" role={interactive ? 'radiogroup' : 'img'} aria-label={`${value.toFixed(1)} out of 5 stars`}>
      {[1, 2, 3, 4, 5].map((n) => {
        const icon = (
          <Star
            size={size}
            className={n <= rounded ? 'fill-amber-400 text-amber-400' : 'text-stone-300 dark:text-stone-600'}
          />
        )
        if (!interactive) {
          return (
            <span key={n} className="p-0.5">
              {icon}
            </span>
          )
        }
        return (
          <button
            key={n}
            type="button"
            role="radio"
            aria-checked={n === rounded}
            aria-label={`${n} star${n > 1 ? 's' : ''}`}
            onClick={() => onChange?.(n)}
            className="cursor-pointer rounded p-0.5 transition hover:scale-110"
          >
            {icon}
          </button>
        )
      })}
    </div>
  )
}
