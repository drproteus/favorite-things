import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Check, Folder, Globe, ImageOff, Lock, Pencil, Share2, Trash2 } from 'lucide-react'
import { api } from '../api'
import StarRating from '../components/StarRating'
import { ErrorState, Loading } from '../components/Status'
import { pathTo } from '../folders'
import { useCollections, useItem, useMe } from '../hooks'
import { itemToInput, type Item, type Visibility } from '../types'
import { btnDanger, btnGhost, btnPrimary, btnSecondary, card, chip } from '../ui'

export default function ItemDetailPage() {
  const { id = '' } = useParams()
  const { data: item, isPending, error } = useItem(id)

  if (isPending) return <Loading />
  if (error) return <ErrorState message={error.message || 'Item not found or not visible to you.'} />
  return <ItemView item={item} />
}

function ItemView({ item }: { item: Item }) {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const { data: me } = useMe()
  const [active, setActive] = useState(0)
  const [copied, setCopied] = useState(false)

  const isOwner = me?.id === item.ownerId
  const isPublic = item.visibility === 'public'
  const current = item.images[active]
  const folders = useCollections(isOwner)
  const folderPath = folders.data ? pathTo(folders.data, item.collectionId) : []

  const refresh = () => {
    qc.invalidateQueries({ queryKey: ['items'] })
    qc.invalidateQueries({ queryKey: ['item', item.id] })
  }

  const rate = useMutation({
    mutationFn: (rating: number) => api.setRating(item.id, rating),
    onSuccess: refresh,
  })
  const clearRating = useMutation({
    mutationFn: () => api.clearRating(item.id),
    onSuccess: refresh,
  })
  const toggleVisibility = useMutation({
    mutationFn: (visibility: Visibility) => api.updateItem(item.id, { ...itemToInput(item), visibility }),
    onSuccess: refresh,
  })
  const remove = useMutation({
    mutationFn: () => api.deleteItem(item.id),
    onSuccess: () => {
      qc.removeQueries({ queryKey: ['item', item.id] })
      qc.invalidateQueries({ queryKey: ['items'] })
      navigate('/collection')
    },
  })

  const share = async () => {
    try {
      await navigator.clipboard.writeText(window.location.href)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Clipboard may be unavailable (e.g. insecure context); the URL is visible in the address bar.
    }
  }

  const confirmDelete = () => {
    if (window.confirm(`Delete "${item.title}"? This cannot be undone.`)) remove.mutate()
  }

  const busy = rate.isPending || clearRating.isPending || toggleVisibility.isPending || remove.isPending
  const mutationError = [rate, clearRating, toggleVisibility, remove].find((m) => m.error)?.error

  return (
    <div className="space-y-6">
      <Link
        to={isOwner ? '/collection' : '/explore'}
        className="inline-flex items-center gap-1 text-sm text-stone-500 hover:text-indigo-600 dark:text-stone-400"
      >
        <ArrowLeft size={16} /> Back
      </Link>

      <div className="grid gap-8 lg:grid-cols-5">
        <section className="space-y-3 lg:col-span-3">
          <div className="flex aspect-4/3 items-center justify-center overflow-hidden rounded-2xl bg-stone-100 dark:bg-stone-800">
            {current ? (
              <img src={current.url} alt={item.title} className="h-full w-full object-contain" />
            ) : (
              <div className="flex flex-col items-center gap-2 text-stone-400">
                <ImageOff size={40} />
                <span className="text-sm">No photos yet</span>
              </div>
            )}
          </div>
          {item.images.length > 1 && (
            <div className="flex gap-2 overflow-x-auto pb-1">
              {item.images.map((img, i) => (
                <button
                  key={img.id}
                  type="button"
                  onClick={() => setActive(i)}
                  aria-label={`Show photo ${i + 1}`}
                  className={`h-16 w-16 shrink-0 cursor-pointer overflow-hidden rounded-lg border-2 transition ${
                    i === active ? 'border-indigo-500' : 'border-transparent opacity-70 hover:opacity-100'
                  }`}
                >
                  <img src={img.url} alt="" className="h-full w-full object-cover" />
                </button>
              ))}
            </div>
          )}
        </section>

        <section className="space-y-6 lg:col-span-2">
          <div className="space-y-3">
            <div className="flex flex-wrap items-center gap-2">
              <span
                className={`inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium ${
                  isPublic
                    ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300'
                    : 'bg-stone-100 text-stone-600 dark:bg-stone-800 dark:text-stone-300'
                }`}
              >
                {isPublic ? <Globe size={12} /> : <Lock size={12} />}
                {isPublic ? 'Public' : 'Private'}
              </span>
              {isOwner && folderPath.length > 0 && (
                <Link to={`/collection?folder=${item.collectionId}`} className={`${chip} inline-flex items-center gap-1`}>
                  <Folder size={12} />
                  {folderPath.map((f) => (f.parentId === null ? 'My collection' : f.name)).join(' / ')}
                </Link>
              )}
            </div>
            <h1 className="text-3xl font-bold tracking-tight">{item.title}</h1>
            <p className="text-sm text-stone-500 dark:text-stone-400">
              Added by {item.ownerName} · updated {new Date(item.updatedAt).toLocaleDateString()}
            </p>
          </div>

          {(item.rating !== null || isOwner) && (
            <div className={`${card} p-5`}>
              <div className="text-sm text-stone-500 dark:text-stone-400">Rating</div>
              <div className="mt-1 flex items-center gap-2">
                {isOwner ? (
                  <>
                    <StarRating
                      value={item.rating ?? 0}
                      onChange={(n) => rate.mutate(n)}
                      disabled={busy}
                      size={22}
                    />
                    {item.rating !== null && (
                      <button type="button" onClick={() => clearRating.mutate()} disabled={busy} className={btnGhost}>
                        Clear
                      </button>
                    )}
                  </>
                ) : (
                  <StarRating value={item.rating ?? 0} size={20} />
                )}
              </div>
            </div>
          )}

          {item.description && (
            <div className={`${card} p-5`}>
              <h2 className="mb-2 text-sm font-semibold text-stone-500 dark:text-stone-400">Description</h2>
              <p className="whitespace-pre-wrap leading-relaxed text-stone-800 dark:text-stone-200">{item.description}</p>
            </div>
          )}

          {item.tags.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {item.tags.map((t) => (
                <span key={t} className={chip}>
                  #{t}
                </span>
              ))}
            </div>
          )}

          {mutationError && (
            <p role="alert" className="text-sm text-rose-600 dark:text-rose-400">
              {mutationError.message}
            </p>
          )}

          <div className="flex flex-wrap gap-2">
            {isOwner && (
              <>
                <Link to={`/items/${item.id}/edit`} className={btnPrimary}>
                  <Pencil size={16} /> Edit
                </Link>
                <button
                  type="button"
                  onClick={() => toggleVisibility.mutate(isPublic ? 'private' : 'public')}
                  disabled={busy}
                  className={btnSecondary}
                >
                  {isPublic ? <Lock size={16} /> : <Globe size={16} />}
                  {isPublic ? 'Make private' : 'Make public'}
                </button>
                <button type="button" onClick={confirmDelete} disabled={busy} className={btnDanger}>
                  <Trash2 size={16} /> Delete
                </button>
              </>
            )}
            {isPublic && (
              <button type="button" onClick={share} className={btnSecondary}>
                {copied ? <Check size={16} /> : <Share2 size={16} />}
                {copied ? 'Link copied' : 'Copy share link'}
              </button>
            )}
          </div>
        </section>
      </div>
    </div>
  )
}
