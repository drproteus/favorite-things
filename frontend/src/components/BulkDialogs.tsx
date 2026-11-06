import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api'
import { flattenFolders, rootFolder } from '../folders'
import type { Collection, DeleteContents, Visibility } from '../types'
import { btnDanger, btnPrimary, btnSecondary, input, label } from '../ui'
import { Modal } from './FolderDialogs'

interface Picked {
  itemIds: string[]
  folderIds: string[]
}

function countText({ itemIds, folderIds }: Picked) {
  const parts = []
  if (itemIds.length) parts.push(`${itemIds.length} item${itemIds.length === 1 ? '' : 's'}`)
  if (folderIds.length) parts.push(`${folderIds.length} folder${folderIds.length === 1 ? '' : 's'}`)
  return parts.join(' and ')
}

export function BulkMoveDialog({
  folders,
  picked,
  defaultFolderId,
  onClose,
  onDone,
}: {
  folders: Collection[]
  picked: Picked
  defaultFolderId: string
  onClose: () => void
  onDone: () => void
}) {
  const qc = useQueryClient()
  const root = rootFolder(folders)
  // A selected folder can't be a destination, nor can anything inside it.
  const options = flattenFolders(folders, picked.folderIds)
  const [destination, setDestination] = useState(
    options.some((o) => o.folder.id === defaultFolderId) ? defaultFolderId : (root?.id ?? ''),
  )

  const move = useMutation({
    mutationFn: async () => {
      if (picked.folderIds.length) await api.bulkMoveCollections(picked.folderIds, destination)
      if (picked.itemIds.length) await api.bulkMoveItems(picked.itemIds, destination)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['collections'] })
      qc.invalidateQueries({ queryKey: ['items'] })
      onDone()
    },
  })

  return (
    <Modal title={`Move ${countText(picked)}`} onClose={onClose}>
      <div className="space-y-5">
        <div>
          <label htmlFor="bulk-destination" className={label}>
            Move to
          </label>
          <select
            id="bulk-destination"
            className={input}
            value={destination}
            onChange={(e) => setDestination(e.target.value)}
          >
            {options.map(({ folder, depth }) => (
              <option key={folder.id} value={folder.id}>
                {'\u2003'.repeat(depth)}
                {folder.parentId === null ? 'My collection' : folder.name}
              </option>
            ))}
          </select>
        </div>

        {move.error && (
          <p role="alert" className="text-sm text-rose-600 dark:text-rose-400">
            {move.error.message}
          </p>
        )}

        <div className="flex justify-end gap-3">
          <button type="button" onClick={onClose} className={btnSecondary}>
            Cancel
          </button>
          <button type="button" onClick={() => move.mutate()} disabled={move.isPending || !destination} className={btnPrimary}>
            {move.isPending ? 'Moving…' : 'Move'}
          </button>
        </div>
      </div>
    </Modal>
  )
}

export function BulkVisibilityDialog({
  picked,
  onClose,
  onDone,
}: {
  picked: Picked
  onClose: () => void
  onDone: () => void
}) {
  const qc = useQueryClient()
  const [visibility, setVisibility] = useState<Visibility>('public')
  const [recursive, setRecursive] = useState(false)
  const hasFolders = picked.folderIds.length > 0

  const change = useMutation({
    mutationFn: async () => {
      if (picked.folderIds.length) await api.bulkVisibilityCollections(picked.folderIds, visibility, recursive)
      if (picked.itemIds.length) await api.bulkVisibilityItems(picked.itemIds, visibility)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['collections'] })
      qc.invalidateQueries({ queryKey: ['items'] })
      onDone()
    },
  })

  return (
    <Modal title={`Change visibility of ${countText(picked)}`} onClose={onClose}>
      <div className="space-y-5">
        <div>
          <span className={label}>Visibility</span>
          <div role="radiogroup" aria-label="Visibility" className="grid grid-cols-2 gap-1 rounded-lg bg-stone-100 p-1 dark:bg-stone-800">
            {(['public', 'private'] as const).map((v) => (
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
        </div>

        {hasFolders && (
          <label className="flex cursor-pointer items-start gap-3 rounded-xl border border-stone-200 p-3 dark:border-stone-700">
            <input type="checkbox" className="mt-1" checked={recursive} onChange={(e) => setRecursive(e.target.checked)} />
            <span>
              <span className="block text-sm font-medium">Also apply to everything inside the selected folders</span>
              <span className="block text-xs text-stone-500 dark:text-stone-400">
                Subfolders and every item in them change too.
              </span>
            </span>
          </label>
        )}

        {change.error && (
          <p role="alert" className="text-sm text-rose-600 dark:text-rose-400">
            {change.error.message}
          </p>
        )}

        <div className="flex justify-end gap-3">
          <button type="button" onClick={onClose} className={btnSecondary}>
            Cancel
          </button>
          <button type="button" onClick={() => change.mutate()} disabled={change.isPending} className={btnPrimary}>
            {change.isPending ? 'Saving…' : 'Apply'}
          </button>
        </div>
      </div>
    </Modal>
  )
}

export function BulkDeleteDialog({
  picked,
  onClose,
  onDone,
}: {
  picked: Picked
  onClose: () => void
  onDone: () => void
}) {
  const qc = useQueryClient()
  const [contents, setContents] = useState<DeleteContents>('delete')
  const hasFolders = picked.folderIds.length > 0

  const remove = useMutation({
    mutationFn: async () => {
      if (picked.itemIds.length) await api.bulkDeleteItems(picked.itemIds)
      if (hasFolders) await api.bulkDeleteCollections(picked.folderIds, contents)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['collections'] })
      qc.invalidateQueries({ queryKey: ['items'] })
      onDone()
    },
  })

  const options: { value: DeleteContents; title: string; text: string }[] = [
    {
      value: 'delete',
      title: 'Delete everything inside',
      text: 'The folders are deleted along with their items, photos and subfolders.',
    },
    {
      value: 'move',
      title: 'Move the contents up one level',
      text: 'Items and subfolders move into the folder each one is in. Only the selected folders are deleted.',
    },
  ]

  return (
    <Modal title={`Delete ${countText(picked)}?`} onClose={onClose}>
      <div className="space-y-4">
        <p className="text-sm text-stone-600 dark:text-stone-300">
          Deleted items and photos can't be recovered.
        </p>

        {hasFolders && (
          <fieldset className="space-y-2">
            <legend className="mb-2 text-sm font-medium">Folder contents</legend>
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
                  name="bulk-contents"
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
        )}

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
            {remove.isPending ? 'Deleting…' : 'Delete'}
          </button>
        </div>
      </div>
    </Modal>
  )
}
