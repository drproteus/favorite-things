import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { CheckSquare, ChevronRight, Folder, FolderPlus, Globe, Lock, MoveRight, Pencil, Plus, Trash2 } from 'lucide-react'
import { BulkDeleteDialog, BulkMoveDialog, BulkVisibilityDialog } from '../components/BulkDialogs'
import { DeleteFolderDialog, FolderFormDialog } from '../components/FolderDialogs'
import FolderHeader from '../components/FolderHeader'
import FolderTree from '../components/FolderTree'
import ItemBrowser from '../components/ItemBrowser'
import { SelectMark } from '../components/ItemCard'
import { ErrorState, Loading } from '../components/Status'
import { childrenOf, pathTo, rootFolder } from '../folders'
import { useCollections } from '../hooks'
import type { Collection, Visibility } from '../types'
import { btnGhost, btnPrimary, btnSecondary, card } from '../ui'

type Dialog = 'new' | 'edit' | 'delete' | 'bulk-move' | 'bulk-visibility' | 'bulk-delete' | null

interface Picked {
  itemIds: string[]
  folderIds: string[]
}

const nameOf = (f: Collection) => (f.parentId === null ? 'My collection' : f.name)

const toggled = (set: string[], id: string) => (set.includes(id) ? set.filter((x) => x !== id) : [...set, id])

export default function CollectionPage() {
  const folders = useCollections()
  const [params, setParams] = useSearchParams()
  const [dialog, setDialog] = useState<Dialog>(null)
  const [selecting, setSelecting] = useState(false)
  const [picked, setPicked] = useState<Picked>({ itemIds: [], folderIds: [] })

  if (folders.isPending) return <Loading />
  if (folders.error) return <ErrorState message={folders.error.message} />

  const list = folders.data
  const root = rootFolder(list)
  if (!root) return <ErrorState message="Your folders could not be loaded." />

  const current = list.find((c) => c.id === params.get('folder')) ?? root
  const isRoot = current.parentId === null
  const path = pathTo(list, current.id)
  const subfolders = childrenOf(list, current.id)

  const clearSelection = () => {
    setSelecting(false)
    setPicked({ itemIds: [], folderIds: [] })
  }
  const go = (id: string) => {
    clearSelection()
    setParams(id === root.id ? {} : { folder: id })
  }
  const finish = () => {
    setDialog(null)
    clearSelection()
  }

  const pickedCount = picked.itemIds.length + picked.folderIds.length

  const actions = (
    <div className="flex flex-wrap items-center gap-2">
      {!selecting && !isRoot && (
        <button type="button" onClick={() => setDialog('edit')} className={btnGhost} aria-label="Edit folder">
          <Pencil size={16} />
        </button>
      )}
      <div className="ml-auto flex flex-wrap items-center justify-end gap-2">
        <button
          type="button"
          onClick={() => (selecting ? clearSelection() : setSelecting(true))}
          className={selecting ? btnPrimary : btnSecondary}
        >
          <CheckSquare size={16} /> {selecting ? 'Done selecting' : 'Select'}
        </button>
        {!selecting && (
          <>
            <button type="button" onClick={() => setDialog('new')} className={btnSecondary}>
              <FolderPlus size={16} /> New folder
            </button>
            <Link to={`/items/new?folder=${current.id}`} className={btnPrimary}>
              <Plus size={16} /> New item
            </Link>
          </>
        )}
      </div>
    </div>
  )

  const breadcrumb = path.length > 1 && (
    <nav aria-label="Breadcrumb" className="flex flex-wrap items-center gap-1 text-sm text-stone-500 dark:text-stone-400">
      {path.map((f, i) => (
        <span key={f.id} className="flex items-center gap-1">
          {i > 0 && <ChevronRight size={14} />}
          {i === path.length - 1 ? (
            <span className="font-medium text-stone-800 dark:text-stone-200">{nameOf(f)}</span>
          ) : (
            <button type="button" onClick={() => go(f.id)} className="cursor-pointer hover:text-indigo-600">
              {nameOf(f)}
            </button>
          )}
        </span>
      ))}
    </nav>
  )

  const folderGrid = (visibility: Visibility | '') => {
    const shown = subfolders.filter((f) => !visibility || f.visibility === visibility)
    return shown.length > 0 && (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      {shown.map((f) => {
        const isPicked = picked.folderIds.includes(f.id)
        const content = (
          <>
            <Folder size={20} className="shrink-0 text-indigo-500" />
            <span className="min-w-0 flex-1">
              <span className="block truncate font-medium">{f.name}</span>
              <span className="flex items-center gap-1 text-xs text-stone-500 dark:text-stone-400">
                {f.visibility === 'public' ? <Globe size={11} /> : <Lock size={11} />}
                {f.visibility === 'public' ? 'Public' : 'Private'}
              </span>
            </span>
          </>
        )
        if (selecting) {
          return (
            <button
              key={f.id}
              type="button"
              aria-pressed={isPicked}
              onClick={() => setPicked((p) => ({ ...p, folderIds: toggled(p.folderIds, f.id) }))}
              className={`${card} relative flex cursor-pointer items-center gap-3 p-4 text-left transition ${isPicked ? 'ring-2 ring-indigo-500' : 'hover:shadow-md'
                }`}
            >
              <SelectMark selected={isPicked} />
              {content}
            </button>
          )
        }
        return (
          <button
            key={f.id}
            type="button"
            onClick={() => go(f.id)}
            className={`${card} flex cursor-pointer items-center gap-3 p-4 text-left transition hover:-translate-y-0.5 hover:shadow-md`}
          >
            {content}
          </button>
        )
      })}
    </div>
    )
  }

  return (
    <div className="flex gap-8">
      <aside className="sticky top-4 hidden w-56 shrink-0 self-start lg:block">
        <FolderTree folders={list} currentId={current.id} onSelect={go} />
      </aside>

      <div className="min-w-0 flex-1 pb-24">
        <ItemBrowser
          scope="mine"
          title={nameOf(current)}
          actions={actions}
          collectionId={current.id}
          before={
            <div className="space-y-4">
              {breadcrumb}
              {!isRoot && <FolderHeader folder={current} />}
            </div>
          }
          folders={folderGrid}
          selection={{
            active: selecting,
            selected: new Set(picked.itemIds),
            onToggle: (id) => setPicked((p) => ({ ...p, itemIds: toggled(p.itemIds, id) })),
          }}
        />
      </div>

      {selecting && (
        <div
          role="toolbar"
          aria-label="Selection actions"
          className="fixed inset-x-0 bottom-6 z-40 mx-auto flex w-fit max-w-[calc(100%-2rem)] flex-wrap items-center gap-3 rounded-2xl bg-stone-900 px-5 py-3 text-sm text-white shadow-xl dark:bg-stone-100 dark:text-stone-900"
        >
          <span className="font-medium">
            {pickedCount === 0 ? 'Select items or folders' : `${pickedCount} selected`}
          </span>
          <button
            type="button"
            onClick={() => setDialog('bulk-move')}
            disabled={pickedCount === 0}
            className="inline-flex cursor-pointer items-center gap-1.5 rounded-lg bg-white/15 px-3 py-1.5 font-medium transition hover:bg-white/25 disabled:cursor-not-allowed disabled:opacity-40 dark:bg-stone-900/10 dark:hover:bg-stone-900/20"
          >
            <MoveRight size={15} /> Move
          </button>
          <button
            type="button"
            onClick={() => setDialog('bulk-visibility')}
            disabled={pickedCount === 0}
            className="inline-flex cursor-pointer items-center gap-1.5 rounded-lg bg-white/15 px-3 py-1.5 font-medium transition hover:bg-white/25 disabled:cursor-not-allowed disabled:opacity-40 dark:bg-stone-900/10 dark:hover:bg-stone-900/20"
          >
            <Globe size={15} /> Visibility
          </button>
          <button
            type="button"
            onClick={() => setDialog('bulk-delete')}
            disabled={pickedCount === 0}
            className="inline-flex cursor-pointer items-center gap-1.5 rounded-lg bg-rose-600 px-3 py-1.5 font-medium text-white transition hover:bg-rose-700 disabled:cursor-not-allowed disabled:opacity-40"
          >
            <Trash2 size={15} /> Delete
          </button>
          <button
            type="button"
            onClick={clearSelection}
            className="cursor-pointer rounded-lg px-3 py-1.5 font-medium opacity-80 hover:opacity-100"
          >
            Cancel
          </button>
        </div>
      )}

      {dialog === 'new' && (
        <FolderFormDialog folders={list} defaultParentId={current.id} onClose={() => setDialog(null)} />
      )}
      {dialog === 'edit' && !isRoot && (
        <FolderFormDialog
          folders={list}
          folder={current}
          defaultParentId={current.parentId ?? root.id}
          onClose={() => setDialog(null)}
          onDelete={() => setDialog('delete')}
        />
      )}
      {dialog === 'delete' && !isRoot && (
        <DeleteFolderDialog
          folders={list}
          folder={current}
          onDeleted={(parentId) => {
            setDialog(null)
            go(parentId || root.id)
          }}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog === 'bulk-move' && (
        <BulkMoveDialog
          folders={list}
          picked={picked}
          defaultFolderId={current.id}
          onClose={() => setDialog(null)}
          onDone={finish}
        />
      )}
      {dialog === 'bulk-visibility' && (
        <BulkVisibilityDialog picked={picked} onClose={() => setDialog(null)} onDone={finish} />
      )}
      {dialog === 'bulk-delete' && (
        <BulkDeleteDialog picked={picked} onClose={() => setDialog(null)} onDone={finish} />
      )}
    </div>
  )
}
