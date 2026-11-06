import { Link } from 'react-router-dom'
import { Globe, ImageOff, Lock } from 'lucide-react'
import type { Item } from '../types'
import { card, chip } from '../ui'
import StarRating from './StarRating'

export default function ItemCard({ item }: { item: Item }) {
  const cover = item.images[0]?.url
  const isPublic = item.visibility === 'public'

  return (
    <Link
      to={`/items/${item.id}`}
      className={`group ${card} flex flex-col overflow-hidden transition hover:-translate-y-0.5 hover:shadow-md`}
    >
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

        {item.category && (
          <p className="text-xs font-medium uppercase tracking-wide text-stone-500 dark:text-stone-400">{item.category}</p>
        )}

        <div className="mt-auto flex flex-wrap items-center gap-2 pt-1">
          {item.ratingCount > 0 && <StarRating value={item.avgRating} size={14} />}
          {item.tags.slice(0, 3).map((t) => (
            <span key={t} className={chip}>
              {t}
            </span>
          ))}
        </div>
      </div>
    </Link>
  )
}
