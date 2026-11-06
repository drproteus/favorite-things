import { useDeferredValue, useState, type ReactNode } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Compass, Plus, Search } from 'lucide-react'
import { useItems } from '../hooks'
import type { ItemFilter, Visibility } from '../types'
import { btnGhost, btnPrimary, chip, input } from '../ui'
import ItemCard from './ItemCard'
import { EmptyState, ErrorState, Loading } from './Status'
import Segmented from './Segmented'

export interface Selection {
  active: boolean
  selected: Set<string>
  onToggle: (id: string) => void
}

type SearchScope = 'folder' | 'all'

interface Props {
  scope: ItemFilter['scope']
  title: string
  actions?: ReactNode
  // Set when the list is a folder: enables the "This folder / All folders" search scope.
  collectionId?: string
  // One user's public items, for their profile.
  owner?: string
  // Shown between the header and the search toolbar, e.g. a breadcrumb.
  before?: ReactNode
  // Subfolders, shown while not searching. Receives the visibility filter so folders can follow it.
  folders?: (visibility: Visibility | '') => ReactNode
  selection?: Selection
}

export default function ItemBrowser({ scope, title, actions, collectionId, owner, before, folders, selection }: Props) {
  const mine = scope === 'mine'
  const [q, setQ] = useState('')
  const [searchScope, setSearchScope] = useState<SearchScope>('folder')
  const [visibility, setVisibility] = useState<Visibility | ''>('')
  const [params, setParams] = useSearchParams()
  // On Explore the tag lives in the URL, so a tag link can open the page already filtered.
  const tag = mine ? '' : (params.get('tag') ?? '')
  const deferredQ = useDeferredValue(q)
  const searching = deferredQ.trim() !== ''
  const inFolder = collectionId !== undefined && !(searching && searchScope === 'all')

  const filter: ItemFilter = {
    scope,
    q: deferredQ.trim(),
    tag,
    owner,
    collectionId: inFolder ? collectionId : undefined,
    visibility: mine ? visibility : '',
  }
  const items = useItems(filter)

  const folderNodes = !searching && folders ? folders(mine ? visibility : '') : null
  const folderScoped = collectionId !== undefined

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">{title}</h1>
        {actions}
      </div>

      {before}

      <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
        <div className="relative min-w-0 flex-1">
          <Search size={16} className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-stone-400" />
          <input
            className={`${input} pl-9`}
            placeholder={inFolder ? 'Search this folder: titles, descriptions, tags' : 'Search everything: titles, descriptions, tags'}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            aria-label="Search"
          />
        </div>
        {folderScoped && (
          <Segmented<SearchScope>
            label="Search scope"
            value={searchScope}
            onChange={setSearchScope}
            options={[
              { value: 'folder', text: 'This folder' },
              { value: 'all', text: 'All folders' },
            ]}
          />
        )}
        {mine && (
          <Segmented<Visibility | ''>
            label="Visibility"
            value={visibility}
            onChange={setVisibility}
            options={[
              { value: '', text: 'All' },
              { value: 'public', text: 'Public' },
              { value: 'private', text: 'Private' },
            ]}
          />
        )}
      </div>

      {!mine && tag && (
        <div className="flex items-center gap-2 text-sm text-stone-600 dark:text-stone-300">
          Showing <span className={chip}>#{tag}</span>
          <button type="button" onClick={() => setParams({})} className={btnGhost}>
            Clear
          </button>
        </div>
      )}

      {folderNodes}

      {items.isPending && <Loading />}
      {items.isError && <ErrorState message={items.error.message} />}
      {items.data &&
        (items.data.length === 0 ? (
          !folderNodes && (
            <EmptyState icon={<Compass size={40} />} title={mine && !searching ? 'Nothing here yet' : 'No items found'}>
              {searching ? (
                'Try a different search.'
              ) : mine ? (
                <Link to="/items/new" className={`${btnPrimary} mt-3`}>
                  <Plus size={16} /> Add your first item
                </Link>
              ) : (
                'Try a different search, or make some of your items public.'
              )}
            </EmptyState>
          )
        ) : (
          <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {items.data.map((item) => (
              <ItemCard
                key={item.id}
                item={item}
                select={
                  selection?.active
                    ? {
                        selected: selection.selected.has(item.id),
                        onToggle: () => selection.onToggle(item.id),
                      }
                    : undefined
                }
              />
            ))}
          </div>
        ))}
    </div>
  )
}
