import { useState, type ChangeEvent, type DragEvent, type FormEvent } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ImageOff, Sparkles, Upload, X } from 'lucide-react'
import { api } from '../api'
import { ErrorState, Loading } from '../components/Status'
import { flattenFolders, rootFolder } from '../folders'
import VisibilityToggle from '../components/VisibilityToggle'
import { useCollections, useItem, useMe } from '../hooks'
import { itemToInput, type Collection, type Item, type ItemInput, type Visibility } from '../types'
import { Modal } from '../components/FolderDialogs'
import { btnDanger, btnDangerMuted, btnPrimary, btnSecondary, card, input, label } from '../ui'

const MAX_IMAGES = 20
const MAX_IMAGE_BYTES = 10 * 1024 * 1024
const ACCEPTED_TYPES = ['image/jpeg', 'image/png', 'image/gif', 'image/webp']

// Adds #hashtags that the description doesn't already contain.
function addMissingTags(description: string, tags: string[]) {
  const missing = tags.map((t) => `#${t}`).filter((t) => !description.toLowerCase().includes(t))
  return [description.trim(), ...missing].filter(Boolean).join(' ')
}

interface PendingFile {
  file: File
  url: string
}

export default function ItemFormPage() {
  const { id } = useParams()
  const [params] = useSearchParams()
  const editing = id !== undefined
  const existing = useItem(id ?? '', editing)
  const folders = useCollections()
  const me = useMe()

  if (editing && existing.isPending) return <Loading />
  if (editing && existing.error) return <ErrorState message={existing.error.message} />
  if (folders.isPending) return <Loading />
  if (folders.error) return <ErrorState message={folders.error.message} />

  const list = folders.data
  const requested = params.get('folder')
  const defaultFolder = list.find((c) => c.id === requested)?.id ?? rootFolder(list)?.id ?? ''
  return (
    <ItemForm
      key={id ?? 'new'}
      existing={existing.data}
      folders={list}
      defaultFolder={defaultFolder}
      defaultVisibility={me.data?.defaultVisibility ?? 'public'}
    />
  )
}

function ItemForm({
  existing,
  folders,
  defaultFolder,
  defaultVisibility,
}: {
  existing?: Item
  folders: Collection[]
  defaultFolder: string
  defaultVisibility: Visibility
}) {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [form, setForm] = useState<ItemInput>(() =>
    existing
      ? itemToInput(existing)
      : { title: '', description: '', collectionId: defaultFolder, visibility: defaultVisibility },
  )
  const [itemId, setItemId] = useState<string | undefined>(existing?.id)
  const [pending, setPending] = useState<PendingFile[]>([])
  const [fileError, setFileError] = useState('')
  const [dragging, setDragging] = useState(false)
  const [suggesting, setSuggesting] = useState(false)
  const [suggestError, setSuggestError] = useState('')

  const existingImages = existing?.images ?? []
  const imageCount = existingImages.length + pending.length

  const update = <K extends keyof ItemInput>(key: K, value: ItemInput[K]) =>
    setForm((f) => ({ ...f, [key]: value }))

  // Fills in only the fields that are still empty, so nothing already typed is overwritten.
  const suggestFrom = async (file: File, quiet = false) => {
    setSuggesting(true)
    setSuggestError('')
    try {
      const s = await api.suggestItem(file)
      setForm((f) => ({
        ...f,
        title: f.title.trim() ? f.title : s.title,
        description: f.description.trim() ? addMissingTags(f.description, s.tags) : s.description,
      }))
    } catch (err) {
      if (!quiet) setSuggestError(err instanceof Error ? err.message : 'Photo suggestion failed')
    } finally {
      setSuggesting(false)
    }
  }

  const queueFiles = (picked: File[]) => {
    const accepted: PendingFile[] = []
    const errors: string[] = []
    for (const file of picked) {
      if (!ACCEPTED_TYPES.includes(file.type)) errors.push(`${file.name}: unsupported type`)
      else if (file.size > MAX_IMAGE_BYTES) errors.push(`${file.name}: larger than 10 MB`)
      else accepted.push({ file, url: URL.createObjectURL(file) })
    }
    const room = MAX_IMAGES - imageCount
    if (accepted.length > room) {
      errors.push(`only ${Math.max(room, 0)} more photo(s) allowed`)
      accepted.splice(Math.max(room, 0)).forEach((p) => URL.revokeObjectURL(p.url))
    }
    setFileError(errors.join('; '))
    setPending((p) => [...p, ...accepted])
    const firstPhoto = existing === undefined && imageCount === 0 && accepted.length > 0
    if (firstPhoto && !form.title.trim() && !form.description.trim()) {
      void suggestFrom(accepted[0].file, true)
    }
  }

  const onPickFiles = (e: ChangeEvent<HTMLInputElement>) => {
    queueFiles(Array.from(e.target.files ?? []))
    e.target.value = ''
  }

  const onDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setDragging(false)
    queueFiles(Array.from(e.dataTransfer.files).filter((f) => f.type.startsWith('image/')))
  }

  const onDragOver = (e: DragEvent<HTMLDivElement>) => {
    if (!Array.from(e.dataTransfer.types).includes('Files')) return
    e.preventDefault()
    e.dataTransfer.dropEffect = imageCount >= MAX_IMAGES ? 'none' : 'copy'
    setDragging(true)
  }

  const removePending = (item: PendingFile) => {
    URL.revokeObjectURL(item.url)
    setPending((list) => list.filter((p) => p !== item))
  }

  const deleteExisting = useMutation({
    mutationFn: (imageId: string) => api.deleteImage(existing!.id, imageId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['item', existing?.id] }),
  })

  const [confirmDelete, setConfirmDelete] = useState(false)
  const remove = useMutation({
    mutationFn: () => api.deleteItem(existing!.id),
    onSuccess: () => {
      qc.removeQueries({ queryKey: ['item', existing?.id] })
      qc.invalidateQueries({ queryKey: ['items'] })
      navigate('/collection')
    },
  })

  // Saves the item, then uploads any queued photos. If an upload fails, the item keeps its id,
  // so retrying updates it instead of creating a duplicate.
  const save = useMutation({
    mutationFn: async () => {
      let savedId = itemId
      if (savedId) {
        await api.updateItem(savedId, form)
      } else {
        const created = await api.createItem(form)
        savedId = created.id
        setItemId(savedId)
      }
      for (const p of pending) {
        await api.uploadImage(savedId, p.file)
        URL.revokeObjectURL(p.url)
        setPending((list) => list.filter((x) => x !== p))
      }
      return savedId
    },
    onSuccess: (savedId) => {
      qc.invalidateQueries({ queryKey: ['items'] })
      qc.invalidateQueries({ queryKey: ['tags'] })
      qc.invalidateQueries({ queryKey: ['item', savedId] })
      navigate(`/items/${savedId}`)
    },
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    save.mutate()
  }

  return (
    <div className="mx-auto max-w-3xl">
      <h1 className="mb-6 text-2xl font-bold tracking-tight sm:text-3xl">
        {existing ? 'Edit item' : 'New item'}
      </h1>

      <form onSubmit={onSubmit} className={`${card} space-y-7 p-6 sm:p-8`}>
        <div>
          <label htmlFor="title" className={label}>
            Title
          </label>
          <input
            id="title"
            className={input}
            required
            maxLength={200}
            placeholder="e.g. Nintendo Game Boy Color, Blue"
            value={form.title}
            onChange={(e) => update('title', e.target.value)}
          />
        </div>

        <div className="grid gap-6 sm:grid-cols-2">
          <div>
            <label htmlFor="folder" className={label}>
              Folder
            </label>
            <select
              id="folder"
              className={input}
              value={form.collectionId}
              onChange={(e) => update('collectionId', e.target.value)}
            >
              {flattenFolders(folders).map(({ folder, depth }) => (
                <option key={folder.id} value={folder.id}>
                  {'\u2003'.repeat(depth)}
                  {folder.parentId === null ? 'My collection' : folder.name}
                </option>
              ))}
            </select>
          </div>
          <div>
            <span className={label}>Visibility</span>
            <VisibilityToggle value={form.visibility} onChange={(v) => update('visibility', v)} />
          </div>
        </div>

        <div>
          <label htmlFor="description" className={label}>
            Description
          </label>
          <textarea
            id="description"
            className={`${input} min-h-32 resize-y`}
            rows={5}
            maxLength={10000}
            placeholder="Condition, where you got it, why you love it… #vintage #mint"
            value={form.description}
            onChange={(e) => update('description', e.target.value)}
          />
          <p className="mt-1.5 text-xs text-stone-500 dark:text-stone-400">
            Add tags as #hashtags anywhere in the description.
          </p>
        </div>

        <div
          className={`space-y-3 rounded-xl p-3 transition ${
            dragging ? 'bg-indigo-50 ring-2 ring-indigo-500 ring-dashed dark:bg-indigo-950/40' : ''
          }`}
          onDragOver={onDragOver}
          onDragLeave={(e) => {
            if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setDragging(false)
          }}
          onDrop={onDrop}
        >
          <div className="flex items-center justify-between">
            <span className={label}>Photos</span>
            <span className="text-xs text-stone-500 dark:text-stone-400">
              {imageCount} / {MAX_IMAGES}
            </span>
          </div>

          {existingImages.length + pending.length === 0 && (
            <div className="flex h-24 items-center justify-center gap-2 rounded-xl border border-dashed border-stone-300 text-sm text-stone-400 dark:border-stone-700">
              <ImageOff size={18} /> Drop photos here or use “Add photos”
            </div>
          )}

          {(existingImages.length > 0 || pending.length > 0) && (
            <ul className="grid grid-cols-3 gap-3 sm:grid-cols-4">
              {existingImages.map((img) => (
                <li key={img.id} className="group relative aspect-square overflow-hidden rounded-xl bg-stone-100 dark:bg-stone-800">
                  <img src={img.url} alt="" className="h-full w-full object-cover" />
                  <button
                    type="button"
                    onClick={() => deleteExisting.mutate(img.id)}
                    disabled={deleteExisting.isPending}
                    aria-label="Remove photo"
                    className="absolute top-1.5 right-1.5 cursor-pointer rounded-full bg-black/60 p-1 text-white opacity-100 transition hover:bg-rose-600 sm:opacity-0 sm:group-hover:opacity-100"
                  >
                    <X size={14} />
                  </button>
                </li>
              ))}
              {pending.map((p) => (
                <li key={p.url} className="group relative aspect-square overflow-hidden rounded-xl ring-2 ring-indigo-400/60 bg-stone-100 dark:bg-stone-800">
                  <img src={p.url} alt="" className="h-full w-full object-cover" />
                  <span className="absolute bottom-1.5 left-1.5 rounded bg-indigo-600 px-1.5 py-0.5 text-[10px] font-medium text-white">
                    Pending
                  </span>
                  <button
                    type="button"
                    onClick={() => removePending(p)}
                    aria-label="Discard photo"
                    className="absolute top-1.5 right-1.5 cursor-pointer rounded-full bg-black/60 p-1 text-white opacity-100 transition hover:bg-rose-600 sm:opacity-0 sm:group-hover:opacity-100"
                  >
                    <X size={14} />
                  </button>
                </li>
              ))}
            </ul>
          )}

          <label
            className={`${btnSecondary} ${imageCount >= MAX_IMAGES ? 'pointer-events-none opacity-50' : ''}`}
          >
            <Upload size={16} /> Add photos
            <input
              type="file"
              accept={ACCEPTED_TYPES.join(',')}
              multiple
              className="sr-only"
              onChange={onPickFiles}
              disabled={imageCount >= MAX_IMAGES}
            />
          </label>
          {fileError && <p className="text-sm text-rose-600 dark:text-rose-400">{fileError}</p>}
          <p className="text-xs text-stone-500 dark:text-stone-400">
            Drag and drop or browse. JPEG, PNG, GIF or WebP, up to 10 MB each.
          </p>
          {pending.length > 0 && (
            <div className="flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={() => suggestFrom(pending[0].file)}
                disabled={suggesting}
                className={btnSecondary}
              >
                <Sparkles size={16} /> {suggesting ? 'Reading photo…' : 'Suggest from photo'}
              </button>
              {suggestError && <span className="text-sm text-rose-600 dark:text-rose-400">{suggestError}</span>}
            </div>
          )}
        </div>

        {save.error && (
          <p role="alert" className="rounded-lg bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:bg-rose-950 dark:text-rose-300">
            {itemId && !existing
              ? `Item saved, but a photo failed to upload: ${save.error.message}. Press Save to retry.`
              : save.error.message}
          </p>
        )}

        <div className="flex flex-col gap-3 border-t border-stone-200 pt-6 sm:flex-row sm:items-center dark:border-stone-800">
          {existing && (
            <button
              type="button"
              onClick={() => setConfirmDelete(true)}
              disabled={save.isPending}
              className={`${btnDangerMuted} sm:mr-auto`}
            >
              Delete
            </button>
          )}
          <div className="flex flex-col-reverse gap-3 sm:ml-auto sm:flex-row">
            <button type="button" onClick={() => navigate(-1)} className={btnSecondary}>
              Cancel
            </button>
            <button type="submit" disabled={save.isPending || form.title.trim() === ''} className={btnPrimary}>
              {save.isPending ? 'Saving…' : existing ? 'Save changes' : 'Create item'}
            </button>
          </div>
        </div>
      </form>

      {confirmDelete && existing && (
        <Modal title={`Delete “${existing.title}”?`} onClose={() => setConfirmDelete(false)}>
          <div className="space-y-4">
            <p className="text-sm text-stone-600 dark:text-stone-300">
              This item and its photos will be permanently deleted. This can't be undone.
            </p>
            {remove.error && (
              <p role="alert" className="text-sm text-rose-600 dark:text-rose-400">
                {remove.error.message}
              </p>
            )}
            <div className="flex justify-end gap-3">
              <button type="button" onClick={() => setConfirmDelete(false)} className={btnSecondary}>
                Cancel
              </button>
              <button type="button" onClick={() => remove.mutate()} disabled={remove.isPending} className={btnDanger}>
                {remove.isPending ? 'Deleting…' : 'Delete'}
              </button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  )
}
