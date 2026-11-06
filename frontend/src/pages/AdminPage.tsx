import { useState, type FormEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Check, Pencil, Plus, Shield, Trash2, X } from 'lucide-react'
import { api } from '../api'
import { ErrorState, Loading } from '../components/Status'
import { useCatalog } from '../hooks'
import type { Term, TermKind } from '../types'
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
          Manage the site-wide categories and tags that members pick from. Changes don't alter existing items.
        </p>
      </div>

      {catalog.isPending && <Loading />}
      {catalog.isError && <ErrorState message={catalog.error.message} />}
      {catalog.data && (
        <div className="grid gap-6 lg:grid-cols-2">
          <TermManager kind="categories" title="Categories" terms={catalog.data.categories} />
          <TermManager kind="tags" title="Tags" terms={catalog.data.tags} />
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

  const busy = create.isPending || rename.isPending || remove.isPending
  const error = [create, rename, remove].find((m) => m.error)?.error

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
            <li key={t.id} className="flex items-center gap-2 py-2">
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
