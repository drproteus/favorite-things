import { useState, type ChangeEvent, type DragEvent, type FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Globe, ImageOff, Lock, Upload, X } from 'lucide-react'
import { api } from '../api'
import TagInput from '../components/TagInput'
import { ErrorState, Loading } from '../components/Status'
import { useCatalog, useItem, useTags } from '../hooks'
import { itemToInput, type Item, type ItemInput, type Visibility } from '../types'
import { btnPrimary, btnSecondary, card, input, label } from '../ui'

const MAX_IMAGES = 20
const MAX_IMAGE_BYTES = 10 * 1024 * 1024
const ACCEPTED_TYPES = ['image/jpeg', 'image/png', 'image/gif', 'image/webp']

interface PendingFile {
  file: File
  url: string
}

export default function ItemFormPage() {
  const { id } = useParams()
  const editing = id !== undefined
  const existing = useItem(id ?? '', editing)

  if (editing && existing.isPending) return <Loading />
  if (editing && existing.error) return <ErrorState message={existing.error.message} />
  return <ItemForm key={id ?? 'new'} existing={existing.data} />
}

function ItemForm({ existing }: { existing?: Item }) {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const userTags = useTags()
  const catalog = useCatalog()
  const tagSuggestions = [...new Set([...(catalog.data?.tags.map((t) => t.name) ?? []), ...(userTags.data ?? [])])]

  const [form, setForm] = useState<ItemInput>(() =>
    existing
      ? itemToInput(existing)
      : { title: '', description: '', category: '', visibility: 'private', tags: [] },
  )
  const [itemId, setItemId] = useState<string | undefined>(existing?.id)
  const [pending, setPending] = useState<PendingFile[]>([])
  const [fileError, setFileError] = useState('')
  const [dragging, setDragging] = useState(false)

  const existingImages = existing?.images ?? []
  const imageCount = existingImages.length + pending.length

  const update = <K extends keyof ItemInput>(key: K, value: ItemInput[K]) =>
    setForm((f) => ({ ...f, [key]: value }))

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
            <label htmlFor="category" className={label}>
              Category
            </label>
            <input
              id="category"
              className={input}
              list="category-options"
              maxLength={100}
              placeholder="Consoles, Albums, T-shirts…"
              value={form.category}
              onChange={(e) => update('category', e.target.value)}
            />
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
            placeholder="Condition, where you got it, why you love it…"
            value={form.description}
            onChange={(e) => update('description', e.target.value)}
          />
        </div>

        <div>
          <span className={label}>Tags</span>
          <TagInput value={form.tags} onChange={(t) => update('tags', t)} suggestions={tagSuggestions} />
          <p className="mt-1.5 text-xs text-stone-500 dark:text-stone-400">Press Enter or comma to add a tag.</p>
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
        </div>

        {save.error && (
          <p role="alert" className="rounded-lg bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:bg-rose-950 dark:text-rose-300">
            {itemId && !existing
              ? `Item saved, but a photo failed to upload: ${save.error.message}. Press Save to retry.`
              : save.error.message}
          </p>
        )}

        <div className="flex flex-col-reverse gap-3 border-t border-stone-200 pt-6 sm:flex-row sm:justify-end dark:border-stone-800">
          <button type="button" onClick={() => navigate(-1)} className={btnSecondary}>
            Cancel
          </button>
          <button type="submit" disabled={save.isPending || form.title.trim() === ''} className={btnPrimary}>
            {save.isPending ? 'Saving…' : existing ? 'Save changes' : 'Create item'}
          </button>
        </div>
      </form>

      <datalist id="category-options">
        {catalog.data?.categories.map((c) => (
          <option key={c.id} value={c.name} />
        ))}
      </datalist>
    </div>
  )
}

function VisibilityToggle({ value, onChange }: { value: Visibility; onChange: (v: Visibility) => void }) {
  const options: { value: Visibility; icon: typeof Lock; text: string }[] = [
    { value: 'private', icon: Lock, text: 'Private' },
    { value: 'public', icon: Globe, text: 'Public' },
  ]
  return (
    <div role="radiogroup" aria-label="Visibility" className="grid grid-cols-2 gap-1 rounded-lg bg-stone-100 p-1 dark:bg-stone-800">
      {options.map((o) => {
        const active = value === o.value
        return (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onChange(o.value)}
            className={`flex cursor-pointer items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition ${
              active
                ? 'bg-white text-indigo-700 shadow-sm dark:bg-stone-900 dark:text-indigo-300'
                : 'text-stone-500 hover:text-stone-800 dark:text-stone-400 dark:hover:text-stone-200'
            }`}
          >
            <o.icon size={15} />
            {o.text}
          </button>
        )
      })}
    </div>
  )
}
