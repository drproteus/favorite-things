import { useState, type ChangeEvent, type FormEvent, type ReactNode } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api'
import { useMe } from '../hooks'
import { childrenOf, flattenFolders } from '../folders'
import type { Collection, DeleteContents, Visibility } from '../types'
import { btnDanger, btnDangerMuted, btnPrimary, btnSecondary, card, input, label } from '../ui'
import MarkdownField from './MarkdownField'

export function Modal({ title, children, onClose }: { title: string; children: ReactNode; onClose: () => void }) {
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
  onDelete?: () => void
}

export function FolderFormDialog({ folders, folder, defaultParentId, onClose, onDelete }: FormProps) {
  const qc = useQueryClient()
  const editing = folder !== undefined
  const isRoot = editing && folder.parentId === null

  const [name, setName] = useState(folder?.name ?? '')
  const me = useMe()
  const [visibility, setVisibility] = useState<Visibility>(folder?.visibility ?? me.data?.defaultVisibility ?? 'public')
  const [parentId, setParentId] = useState(folder?.parentId ?? defaultParentId)
  const [recursive, setRecursive] = useState(false)
  const [description, setDescription] = useState(folder?.description ?? '')
  const [banner, setBanner] = useState<string | null>(folder?.bannerUrl ?? null)
  const [bannerBusy, setBannerBusy] = useState(false)
  const [bannerError, setBannerError] = useState('')
  const visibilityChanged = editing && !isRoot && visibility !== folder.visibility

  const onBannerPick = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file || !folder) return
    setBannerBusy(true)
    setBannerError('')
    try {
      const updated = await api.uploadFolderBanner(folder.id, file)
      setBanner(updated.bannerUrl)
      qc.invalidateQueries({ queryKey: ['collections'] })
    } catch (err) {
      setBannerError(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setBannerBusy(false)
    }
  }

  const removeBanner = async () => {
    if (!folder) return
    setBannerBusy(true)
    setBannerError('')
    try {
      await api.deleteFolderBanner(folder.id)
      setBanner(null)
      qc.invalidateQueries({ queryKey: ['collections'] })
    } catch (err) {
      setBannerError(err instanceof Error ? err.message : 'Could not remove the image')
    } finally {
      setBannerBusy(false)
    }
  }

  const save = useMutation({
    mutationFn: () => {
      const input = { name: name.trim(), visibility, parentId, description, recursive: visibilityChanged && recursive }
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
              {flattenFolders(folders, folder ? [folder.id] : []).map(({ folder: f, depth }) => (
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

        {editing && !isRoot && (
          <>
            <MarkdownField
              id="folder-description"
              labelText="Description"
              rows={4}
              maxLength={2000}
              placeholder="What's in this folder?"
              value={description}
              onChange={setDescription}
              help="Optional. Shown at the top of the folder. Supports the same markdown as item descriptions."
            />

            <div className="space-y-2">
              <span className={label}>Banner image</span>
              {banner && <img src={banner} alt="" className="h-28 w-full rounded-xl object-cover" />}
              <div className="flex flex-wrap items-center gap-2">
                <label className={`${btnSecondary} ${bannerBusy ? 'pointer-events-none opacity-50' : ''}`}>
                  {bannerBusy ? 'Working…' : banner ? 'Replace image' : 'Add image'}
                  <input
                    type="file"
                    accept="image/jpeg,image/png,image/gif,image/webp"
                    className="sr-only"
                    onChange={onBannerPick}
                    disabled={bannerBusy}
                  />
                </label>
                {banner && (
                  <button type="button" onClick={removeBanner} disabled={bannerBusy} className={btnSecondary}>
                    Remove
                  </button>
                )}
              </div>
              <p className="text-xs text-stone-500 dark:text-stone-400">
                Shown across the top of the folder. JPEG, PNG, GIF or WebP, up to 10 MB. Saved as soon as you choose it.
              </p>
              {bannerError && (
                <p role="alert" className="text-sm text-rose-600 dark:text-rose-400">
                  {bannerError}
                </p>
              )}
            </div>
          </>
        )}

        {visibilityChanged && (
          <label className="flex cursor-pointer items-start gap-3 rounded-xl border border-stone-200 p-3 dark:border-stone-700">
            <input
              type="checkbox"
              className="mt-1"
              checked={recursive}
              onChange={(e) => setRecursive(e.target.checked)}
            />
            <span>
              <span className="block text-sm font-medium">Also make everything inside {visibility}</span>
              <span className="block text-xs text-stone-500 dark:text-stone-400">
                Applies to subfolders and to every item in them.
              </span>
            </span>
          </label>
        )}

        {save.error && (
          <p role="alert" className="text-sm text-rose-600 dark:text-rose-400">
            {save.error.message}
          </p>
        )}

        <div className="flex items-center gap-3">
          {editing && onDelete && (
            <button type="button" onClick={onDelete} className={btnDangerMuted}>
              Delete
            </button>
          )}
          <div className="ml-auto flex gap-3">
            <button type="button" onClick={onClose} className={btnSecondary}>
              Cancel
            </button>
            <button type="submit" disabled={save.isPending || !name.trim()} className={btnPrimary}>
              {save.isPending ? 'Saving…' : editing ? 'Save' : 'Create folder'}
            </button>
          </div>
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
