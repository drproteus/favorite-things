import { useId, useState, type KeyboardEvent } from 'react'
import { X } from 'lucide-react'
import { chip, input } from '../ui'

interface Props {
  value: string[]
  onChange: (tags: string[]) => void
  suggestions: string[]
}

export default function TagInput({ value, onChange, suggestions }: Props) {
  const [draft, setDraft] = useState('')
  const listId = useId()

  const commit = (raw: string) => {
    const parts = raw
      .split(',')
      .map((s) => s.trim().toLowerCase())
      .filter(Boolean)
    if (parts.length) onChange([...new Set([...value, ...parts])])
    setDraft('')
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      commit(draft)
    } else if (e.key === 'Backspace' && draft === '' && value.length > 0) {
      onChange(value.slice(0, -1))
    }
  }

  return (
    <div className={`${input} flex flex-wrap items-center gap-2`}>
      {value.map((tag) => (
        <span key={tag} className={chip}>
          {tag}
          <button
            type="button"
            onClick={() => onChange(value.filter((t) => t !== tag))}
            aria-label={`Remove tag ${tag}`}
            className="rounded-full p-0.5 hover:bg-indigo-100 dark:hover:bg-indigo-900"
          >
            <X size={12} />
          </button>
        </span>
      ))}
      <input
        list={listId}
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={onKeyDown}
        onBlur={() => commit(draft)}
        placeholder={value.length ? '' : 'Add tags, press Enter…'}
        className="min-w-24 flex-1 bg-transparent outline-none"
      />
      <datalist id={listId}>
        {suggestions
          .filter((s) => !value.includes(s))
          .map((s) => (
            <option key={s} value={s} />
          ))}
      </datalist>
    </div>
  )
}
