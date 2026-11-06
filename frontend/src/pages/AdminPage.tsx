import { useState, type FormEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Check, GripVertical, Pencil, Plus, Shield, Trash2, X } from 'lucide-react'
import { api } from '../api'
import { ErrorState, Loading } from '../components/Status'
import { useCatalog } from '../hooks'
import type { Catalog, Term, TermKind } from '../types'
import { btnGhost, btnPrimary, card, input } from '../ui'

export default function AdminPage() {
  const catalog = useCatalog()

  return (
    <div className="space-y-8">
      <div>
        <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight sm:text-3xl">
          <Shield size={26} className="text-indigo-600 dark:text-indigo-400" /> Admin
        </h1>
        <p className="mt-1 text-sm text-stone-500 dark:text-stone-400">
          Manage the site-wide categories that members pick from. Changes don't alter existing items.
        </p>
      </div>

      {catalog.isPending && <Loading />}
      {catalog.isError && <ErrorState message={catalog.error.message} />}
      {catalog.data && (
        <div className="max-w-2xl">
          <TermManager kind="categories" title="Categories" terms={catalog.data.categories} />
        </div>
      )}
    </div>
  )
}

function TermManager({ kind, title, terms }: { kind: TermKind; title: string; terms: Term[] }) {
  const qc = useQueryClient()
  const [newName, setNewName] = useState('')
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editName, setEditName] = useState('')
  const [dragId, setDragId] = useState<number | null>(null)

  const refresh = () => qc.invalidateQueries({ queryKey: ['catalog'] })

  const create = useMutation({
    mutationFn: (name: string) => api.createTerm(kind, name),
    onSuccess: () => {
      setNewName('')
      refresh()
    },
  })
  const rename = useMutation({
    mutationFn: ({ id, name }: { id: number; name: string }) => api.renameTerm(kind, id, name),
    onSuccess: () => {
      setEditingId(null)
      refresh()
    },
  })
  const remove = useMutation({
    mutationFn: (id: number) => api.deleteTerm(kind, id),
    onSuccess: refresh,
  })
  // Show the new order immediately; roll back if the server rejects it.
  const reorder = useMutation({
    mutationFn: (ids: number[]) => api.reorderTerms(kind, ids),
    onMutate: async (ids) => {
      await qc.cancelQueries({ queryKey: ['catalog'] })
      const prev = qc.getQueryData<Catalog>(['catalog'])
      if (prev) {
        const byId = new Map(prev[kind].map((t) => [t.id, t]))
        qc.setQueryData<Catalog>(['catalog'], { ...prev, [kind]: ids.flatMap((id) => byId.get(id) ?? []) })
      }
      return { prev }
    },
    onError: (_err, _ids, ctx) => {
      if (ctx?.prev) qc.setQueryData(['catalog'], ctx.prev)
    },
    onSettled: refresh,
  })

  const busy = create.isPending || rename.isPending || remove.isPending || reorder.isPending
  const error = [create, rename, remove, reorder].find((m) => m.error)?.error

  const moveBefore = (targetId: number) => {
    if (dragId === null || dragId === targetId) return
    const ids = terms.map((t) => t.id)
    ids.splice(ids.indexOf(dragId), 1)
    ids.splice(ids.indexOf(targetId), 0, dragId)
    reorder.mutate(ids)
  }

  const onAdd = (e: FormEvent) => {
    e.preventDefault()
    if (newName.trim()) create.mutate(newName.trim())
  }

  const startEdit = (t: Term) => {
    setEditingId(t.id)
    setEditName(t.name)
  }

  const confirmRemove = (t: Term) => {
    if (window.confirm(`Delete “${t.name}”? Items already using it keep their value.`)) remove.mutate(t.id)
  }

  return (
    <section className={`${card} flex flex-col p-6`}>
      <div className="mb-4 flex items-baseline justify-between">
        <h2 className="text-lg font-semibold">{title}</h2>
        <span className="text-xs text-stone-500 dark:text-stone-400">{terms.length} total</span>
      </div>

      <form onSubmit={onAdd} className="mb-4 flex gap-2">
        <input
          className={input}
          placeholder={`Add a ${kind === 'categories' ? 'category' : 'tag'}`}
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          aria-label={`New ${kind === 'categories' ? 'category' : 'tag'} name`}
          maxLength={kind === 'categories' ? 50 : 40}
        />
        <button type="submit" disabled={busy || !newName.trim()} className={btnPrimary}>
          <Plus size={16} /> Add
        </button>
      </form>

      {error && (
        <p role="alert" className="mb-3 text-sm text-rose-600 dark:text-rose-400">
          {error.message}
        </p>
      )}

      {terms.length === 0 ? (
        <p className="py-6 text-center text-sm text-stone-500">None yet.</p>
      ) : (
        <ul className="divide-y divide-stone-200 dark:divide-stone-800">
          {terms.map((t) => (
            <li
              key={t.id}
              draggable={editingId !== t.id && !busy}
              onDragStart={(e) => {
                e.dataTransfer.effectAllowed = 'move'
                setDragId(t.id)
              }}
              onDragOver={(e) => {
                if (dragId === null) return
                e.preventDefault()
                e.dataTransfer.dropEffect = 'move'
              }}
              onDrop={(e) => {
                e.preventDefault()
                moveBefore(t.id)
                setDragId(null)
              }}
              onDragEnd={() => setDragId(null)}
              className={`flex items-center gap-2 py-2 ${dragId === t.id ? 'opacity-40' : ''}`}
            >
              {editingId !== t.id && (
                <GripVertical size={16} className="shrink-0 cursor-grab text-stone-400" aria-hidden="true" />
              )}
              {editingId === t.id ? (
                <form
                  className="flex flex-1 gap-2"
                  onSubmit={(e) => {
                    e.preventDefault()
                    if (editName.trim()) rename.mutate({ id: t.id, name: editName.trim() })
                  }}
                >
                  <input
                    className={input}
                    value={editName}
                    onChange={(e) => setEditName(e.target.value)}
                    autoFocus
                    aria-label={`Rename ${t.name}`}
                  />
                  <button type="submit" className={btnGhost} aria-label="Save name" disabled={busy}>
                    <Check size={16} />
                  </button>
                  <button type="button" className={btnGhost} aria-label="Cancel rename" onClick={() => setEditingId(null)}>
                    <X size={16} />
                  </button>
                </form>
              ) : (
                <>
                  <span className="flex-1 truncate text-sm">{t.name}</span>
                  <button type="button" className={btnGhost} aria-label={`Rename ${t.name}`} onClick={() => startEdit(t)} disabled={busy}>
                    <Pencil size={15} />
                  </button>
                  <button type="button" className={`${btnGhost} hover:text-rose-600`} aria-label={`Delete ${t.name}`} onClick={() => confirmRemove(t)} disabled={busy}>
                    <Trash2 size={15} />
                  </button>
                </>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
