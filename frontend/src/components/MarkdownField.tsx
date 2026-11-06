import { useState } from 'react'
import Markdown from './Markdown'
import { input } from '../ui'

interface Props {
  id: string
  labelText: string
  value: string
  onChange: (value: string) => void
  maxLength: number
  rows?: number
  placeholder?: string
  help?: string
}

// A textarea with a Preview tab that shows the text as it will look once saved.
export default function MarkdownField({ id, labelText, value, onChange, maxLength, rows = 5, placeholder, help }: Props) {
  const [previewing, setPreviewing] = useState(false)
  const tabs = [
    { key: 'write', text: 'Write', active: !previewing, select: () => setPreviewing(false) },
    { key: 'preview', text: 'Preview', active: previewing, select: () => setPreviewing(true) },
  ]

  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between gap-3">
        <label htmlFor={id} className="text-sm font-medium text-stone-700 dark:text-stone-300">
          {labelText}
        </label>
        <div role="tablist" aria-label={`${labelText} mode`} className="inline-flex rounded-lg bg-stone-100 p-0.5 text-xs font-medium dark:bg-stone-800">
          {tabs.map((t) => (
            <button
              key={t.key}
              type="button"
              role="tab"
              aria-selected={t.active}
              onClick={t.select}
              className={`cursor-pointer rounded-md px-3 py-1 transition ${
                t.active
                  ? 'bg-white text-indigo-700 shadow-sm dark:bg-stone-900 dark:text-indigo-300'
                  : 'text-stone-500 hover:text-stone-800 dark:text-stone-400 dark:hover:text-stone-200'
              }`}
            >
              {t.text}
            </button>
          ))}
        </div>
      </div>

      {previewing ? (
        <div
          id={id}
          role="tabpanel"
          className="min-h-32 rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-stone-800 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-200"
        >
          {value.trim() ? (
            <Markdown text={value} inert />
          ) : (
            <p className="text-stone-400 dark:text-stone-500">Nothing to preview yet.</p>
          )}
        </div>
      ) : (
        <textarea
          id={id}
          className={`${input} min-h-32 resize-y`}
          rows={rows}
          maxLength={maxLength}
          placeholder={placeholder}
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
      )}

      {help && <p className="mt-1.5 text-xs text-stone-500 dark:text-stone-400">{help}</p>}
    </div>
  )
}
