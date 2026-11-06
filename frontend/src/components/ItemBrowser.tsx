import { useDeferredValue, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { Compass, Plus, Search } from 'lucide-react'
import { useCatalog, useItems, useTags } from '../hooks'
import type { ItemFilter, Visibility } from '../types'
import { btnPrimary, input } from '../ui'
import ItemCard from './ItemCard'
import { EmptyState, ErrorState, Loading } from './Status'

interface Props {
  scope: ItemFilter['scope']
  title: string
  subtitle: string
  actions?: ReactNode
}

export default function ItemBrowser({ scope, title, subtitle, actions }: Props) {
  const mine = scope === 'mine'
  const [q, setQ] = useState('')
  const [tag, setTag] = useState('')
  const [categoryId, setCategoryId] = useState(0)
  const [visibility, setVisibility] = useState<Visibility | ''>('')
  const deferredQ = useDeferredValue(q)

  const filter: ItemFilter = {
    scope,
    q: deferredQ,
    tag,
    categoryId,
    visibility: mine ? visibility : '',
  }
  const items = useItems(filter)
  const tags = useTags(mine)
  const catalog = useCatalog()

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">{title}</h1>
          <p className="mt-1 text-sm text-stone-500 dark:text-stone-400">{subtitle}</p>
        </div>
        {actions}
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-[1fr_12rem_12rem_12rem]">
        <div className="relative">
          <Search size={16} className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-stone-400" />
          <input
            className={`${input} pl-9`}
            placeholder="Search titles and descriptions"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            aria-label="Search"
          />
        </div>
        <div>
          <select
            className={input}
            value={categoryId}
            onChange={(e) => setCategoryId(Number(e.target.value))}
            aria-label="Filter by category"
          >
            <option value={0}>All categories</option>
            {catalog.data?.categories.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </div>
        {mine && (
          <>
            <select
              className={input}
              value={tag}
              onChange={(e) => setTag(e.target.value)}
              aria-label="Filter by tag"
            >
              <option value="">All tags</option>
              {tags.data?.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
            <select
              className={input}
              value={visibility}
              onChange={(e) => setVisibility(e.target.value as Visibility | '')}
              aria-label="Filter by visibility"
            >
              <option value="">Any visibility</option>
              <option value="public">Public</option>
              <option value="private">Private</option>
            </select>
          </>
        )}
      </div>

      {items.isPending && <Loading />}
      {items.isError && <ErrorState message={items.error.message} />}
      {items.data &&
        (items.data.length === 0 ? (
          <EmptyState icon={<Compass size={40} />} title={mine ? 'Nothing here yet' : 'No items found'}>
            {mine ? (
              <Link to="/items/new" className={`${btnPrimary} mt-3`}>
                <Plus size={16} /> Add your first item
              </Link>
            ) : (
              'Try a different search, or make some of your items public.'
            )}
          </EmptyState>
        ) : (
          <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {items.data.map((item) => (
              <ItemCard key={item.id} item={item} />
            ))}
          </div>
        ))}

    </div>
  )
}
