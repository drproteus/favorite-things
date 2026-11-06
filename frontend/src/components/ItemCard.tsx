import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { Check, Globe, ImageOff, Lock } from 'lucide-react'
import type { Item } from '../types'
import { card, chip } from '../ui'
import StarRating from './StarRating'

interface Props {
  item: Item
  select?: { selected: boolean; onToggle: () => void }
}

export default function ItemCard({ item, select }: Props) {
  const cover = item.images[0]?.url
  const isPublic = item.visibility === 'public'

  const body: ReactNode = (
    <>
      <div className="aspect-4/3 bg-stone-100 dark:bg-stone-800">
        {cover ? (
          <img src={cover} alt="" loading="lazy" className="h-full w-full object-cover transition group-hover:scale-105" />
        ) : (
          <div className="flex h-full items-center justify-center text-stone-400">
            <ImageOff size={32} />
          </div>
        )}
      </div>

      <div className="flex flex-1 flex-col gap-2 p-4">
        <div className="flex items-start justify-between gap-2">
          <h3 className="line-clamp-2 font-semibold leading-snug">{item.title}</h3>
          <span
            title={isPublic ? 'Public' : 'Private'}
            className={`mt-0.5 shrink-0 ${isPublic ? 'text-emerald-600 dark:text-emerald-400' : 'text-stone-400'}`}
          >
            {isPublic ? <Globe size={15} /> : <Lock size={15} />}
          </span>
        </div>

        <div className="mt-auto flex flex-wrap items-center gap-2 pt-1">
          {item.rating !== null && <StarRating value={item.rating} size={14} />}
          {item.tags.slice(0, 3).map((t) => (
            <span key={t} className={chip}>
              {t}
            </span>
          ))}
        </div>
      </div>
    </>
  )

  if (select) {
    return (
      <button
        type="button"
        onClick={select.onToggle}
        aria-pressed={select.selected}
        className={`group relative ${card} flex cursor-pointer flex-col overflow-hidden text-left transition ${
          select.selected ? 'ring-2 ring-indigo-500' : 'hover:shadow-md'
        }`}
      >
        <SelectMark selected={select.selected} />
        {body}
      </button>
    )
  }

  return (
    <Link
      to={`/items/${item.id}`}
      className={`group ${card} flex flex-col overflow-hidden transition hover:-translate-y-0.5 hover:shadow-md`}
    >
      {body}
    </Link>
  )
}

export function SelectMark({ selected }: { selected: boolean }) {
  return (
    <span
      aria-hidden="true"
      className={`absolute top-2 left-2 z-10 flex h-6 w-6 items-center justify-center rounded-full border-2 shadow-sm ${
        selected
          ? 'border-indigo-600 bg-indigo-600 text-white'
          : 'border-white bg-white/80 text-transparent dark:border-stone-900 dark:bg-stone-900/80'
      }`}
    >
      <Check size={14} />
    </span>
  )
}
