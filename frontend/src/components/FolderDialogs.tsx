import { useState, type FormEvent, type ReactNode } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api'
import { childrenOf, flattenFolders } from '../folders'
import type { Collection, DeleteContents, Visibility } from '../types'
import { btnDanger, btnPrimary, btnSecondary, card, input, label } from '../ui'

function Modal({ title, children, onClose }: { title: string; children: ReactNode; onClose: () => void }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-stone-900/40 p-4" onMouseDown={onClose}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className={`${card} w-full max-w-md p-6`}
        onMouseDown={(e) => e.stopPropagation()}
      >
        <h2 className="mb-4 text-lg font-semibold">{title}</h2>
        {children}
      </div>
    </div>
  )
}

interface FormProps {
  folders: Collection[]
  folder?: Collection
  defaultParentId: string
  onClose: () => void
}

export function FolderFormDialog({ folders, folder, defaultParentId, onClose }: FormProps) {
  const qc = useQueryClient()
  const editing = folder !== undefined
  const isRoot = editing && folder.parentId === null

  const [name, setName] = useState(folder?.name ?? '')
  const [visibility, setVisibility] = useState<Visibility>(folder?.visibility ?? 'private')
  const [parentId, setParentId] = useState(folder?.parentId ?? defaultParentId)

  const save = useMutation({
    mutationFn: () => {
      const input = { name: name.trim(), visibility, parentId }
      return editing ? api.updateCollection(folder.id, input) : api.createCollection(input)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['collections'] })
      qc.invalidateQueries({ queryKey: ['items'] })
      onClose()
    },
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (name.trim()) save.mutate()
  }

  return (
    <Modal title={editing ? 'Edit folder' : 'New folder'} onClose={onClose}>
      <form onSubmit={onSubmit} className="space-y-5">
        <div>
          <label htmlFor="folder-name" className={label}>
            Name
          </label>
          <input
            id="folder-name"
            className={input}
            required
            maxLength={100}
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </div>

        {!isRoot && (
          <div>
            <label htmlFor="folder-parent" className={label}>
              Inside
            </label>
            <select id="folder-parent" className={input} value={parentId} onChange={(e) => setParentId(e.target.value)}>
              {flattenFolders(folders, folder?.id).map(({ folder: f, depth }) => (
                <option key={f.id} value={f.id}>
                  {'\u2003'.repeat(depth)}
                  {f.parentId === null ? 'My collection' : f.name}
                </option>
              ))}
            </select>
          </div>
        )}

        {!isRoot && (
          <div>
            <span className={label}>Visibility</span>
            <div role="radiogroup" aria-label="Folder visibility" className="grid grid-cols-2 gap-1 rounded-lg bg-stone-100 p-1 dark:bg-stone-800">
              {(['private', 'public'] as const).map((v) => (
                <button
                  key={v}
                  type="button"
                  role="radio"
                  aria-checked={visibility === v}
                  onClick={() => setVisibility(v)}
                  className={`cursor-pointer rounded-md px-3 py-2 text-sm font-medium capitalize transition ${
                    visibility === v
                      ? 'bg-white text-indigo-700 shadow-sm dark:bg-stone-900 dark:text-indigo-300'
                      : 'text-stone-500 dark:text-stone-400'
                  }`}
                >
                  {v}
                </button>
              ))}
            </div>
            <p className="mt-1.5 text-xs text-stone-500 dark:text-stone-400">
              A private folder hides everything inside it from other people, even items marked public.
            </p>
          </div>
        )}

        {save.error && (
          <p role="alert" className="text-sm text-rose-600 dark:text-rose-400">
            {save.error.message}
          </p>
        )}

        <div className="flex justify-end gap-3">
          <button type="button" onClick={onClose} className={btnSecondary}>
            Cancel
          </button>
          <button type="submit" disabled={save.isPending || !name.trim()} className={btnPrimary}>
            {save.isPending ? 'Saving…' : editing ? 'Save' : 'Create folder'}
          </button>
        </div>
      </form>
    </Modal>
  )
}

export function DeleteFolderDialog({
  folders,
  folder,
  onDeleted,
  onClose,
}: {
  folders: Collection[]
  folder: Collection
  onDeleted: (parentId: string) => void
  onClose: () => void
}) {
  const qc = useQueryClient()
  const [contents, setContents] = useState<DeleteContents>('delete')
  const subfolders = childrenOf(folders, folder.id).length

  const remove = useMutation({
    mutationFn: () => api.deleteCollection(folder.id, contents),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['collections'] })
      qc.invalidateQueries({ queryKey: ['items'] })
      onDeleted(folder.parentId ?? '')
    },
  })

  const options: { value: DeleteContents; title: string; text: string }[] = [
    {
      value: 'delete',
      title: 'Delete everything inside',
      text: `Permanently deletes the folder, its photos and items${subfolders ? `, and its ${subfolders} subfolder${subfolders === 1 ? '' : 's'}` : ''}.`,
    },
    {
      value: 'move',
      title: 'Move the contents up one level',
      text: 'Items and subfolders move into the folder this one is in. Only the folder is deleted.',
    },
  ]

  return (
    <Modal title={`Delete “${folder.name}”?`} onClose={onClose}>
      <div className="space-y-4">
        <fieldset className="space-y-2">
          <legend className="sr-only">What to do with the contents</legend>
          {options.map((o) => (
            <label
              key={o.value}
              className={`flex cursor-pointer gap-3 rounded-xl border p-3 transition ${
                contents === o.value
                  ? 'border-indigo-500 bg-indigo-50 dark:bg-indigo-950/40'
                  : 'border-stone-200 dark:border-stone-700'
              }`}
            >
              <input
                type="radio"
                name="contents"
                className="mt-1"
                checked={contents === o.value}
                onChange={() => setContents(o.value)}
              />
              <span>
                <span className="block text-sm font-medium">{o.title}</span>
                <span className="block text-xs text-stone-500 dark:text-stone-400">{o.text}</span>
              </span>
            </label>
          ))}
        </fieldset>

        {remove.error && (
          <p role="alert" className="text-sm text-rose-600 dark:text-rose-400">
            {remove.error.message}
          </p>
        )}

        <div className="flex justify-end gap-3">
          <button type="button" onClick={onClose} className={btnSecondary}>
            Cancel
          </button>
          <button type="button" onClick={() => remove.mutate()} disabled={remove.isPending} className={btnDanger}>
            {remove.isPending ? 'Deleting…' : 'Delete folder'}
          </button>
        </div>
      </div>
    </Modal>
  )
}
