interface Props<T extends string> {
  label: string
  options: { value: T; text: string }[]
  value: T
  onChange: (value: T) => void
}

// A row of mutually exclusive options, shown as a single toggle.
export default function Segmented<T extends string>({ label, options, value, onChange }: Props<T>) {
  return (
    <div
      role="radiogroup"
      aria-label={label}
      className="inline-grid auto-cols-fr grid-flow-col gap-1 rounded-lg bg-stone-100 p-1 dark:bg-stone-800"
    >
      {options.map((o) => {
        const active = o.value === value
        return (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onChange(o.value)}
            className={`cursor-pointer whitespace-nowrap rounded-md px-3 py-2 text-sm font-medium transition ${
              active
                ? 'bg-white text-indigo-700 shadow-sm dark:bg-stone-900 dark:text-indigo-300'
                : 'text-stone-500 hover:text-stone-800 dark:text-stone-400 dark:hover:text-stone-200'
            }`}
          >
            {o.text}
          </button>
        )
      })}
    </div>
  )
}
