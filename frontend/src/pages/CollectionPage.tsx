import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ChevronRight, Folder, FolderPlus, Globe, Lock, Pencil, Plus, Trash2 } from 'lucide-react'
import { DeleteFolderDialog, FolderFormDialog } from '../components/FolderDialogs'
import FolderTree from '../components/FolderTree'
import ItemBrowser from '../components/ItemBrowser'
import { ErrorState, Loading } from '../components/Status'
import { childrenOf, pathTo, rootFolder } from '../folders'
import { useCollections } from '../hooks'
import type { Collection } from '../types'
import { btnGhost, btnPrimary, btnSecondary, card } from '../ui'

type Dialog = 'new' | 'edit' | 'delete' | null

const nameOf = (f: Collection) => (f.parentId === null ? 'My collection' : f.name)

export default function CollectionPage() {
  const folders = useCollections()
  const [params, setParams] = useSearchParams()
  const [dialog, setDialog] = useState<Dialog>(null)

  if (folders.isPending) return <Loading />
  if (folders.error) return <ErrorState message={folders.error.message} />

  const list = folders.data
  const root = rootFolder(list)
  if (!root) return <ErrorState message="Your folders could not be loaded." />

  const current = list.find((c) => c.id === params.get('folder')) ?? root
  const isRoot = current.parentId === null
  const path = pathTo(list, current.id)
  const subfolders = childrenOf(list, current.id)

  const go = (id: string) => setParams(id === root.id ? {} : { folder: id })

  const actions = (
    <>
      <button type="button" onClick={() => setDialog('new')} className={btnSecondary}>
        <FolderPlus size={16} /> New folder
      </button>
      {!isRoot && (
        <>
          <button type="button" onClick={() => setDialog('edit')} className={btnGhost} aria-label="Edit folder">
            <Pencil size={16} />
          </button>
          <button
            type="button"
            onClick={() => setDialog('delete')}
            className={`${btnGhost} hover:text-rose-600`}
            aria-label="Delete folder"
          >
            <Trash2 size={16} />
          </button>
        </>
      )}
      <Link to={`/items/new?folder=${current.id}`} className={btnPrimary}>
        <Plus size={16} /> New item
      </Link>
    </>
  )

  const before = (
    <div className="space-y-4">
      {path.length > 1 && (
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
      )}

      {subfolders.length > 0 && (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {subfolders.map((f) => (
            <button
              key={f.id}
              type="button"
              onClick={() => go(f.id)}
              className={`${card} flex cursor-pointer items-center gap-3 p-4 text-left transition hover:-translate-y-0.5 hover:shadow-md`}
            >
              <Folder size={20} className="shrink-0 text-indigo-500" />
              <span className="min-w-0 flex-1">
                <span className="block truncate font-medium">{f.name}</span>
                <span className="flex items-center gap-1 text-xs text-stone-500 dark:text-stone-400">
                  {f.visibility === 'public' ? <Globe size={11} /> : <Lock size={11} />}
                  {f.visibility === 'public' ? 'Public' : 'Private'}
                </span>
              </span>
            </button>
          ))}
        </div>
      )}
    </div>
  )

  return (
    <div className="flex gap-8">
      <aside className="sticky top-4 hidden w-56 shrink-0 self-start lg:block">
        <FolderTree folders={list} currentId={current.id} onSelect={go} />
      </aside>

      <div className="min-w-0 flex-1">
        <ItemBrowser
          scope="mine"
          title={nameOf(current)}
          subtitle="Items and folders you've cataloged, public or private."
          actions={actions}
          collectionId={current.id}
          before={before}
        />
      </div>

      {dialog === 'new' && (
        <FolderFormDialog folders={list} defaultParentId={current.id} onClose={() => setDialog(null)} />
      )}
      {dialog === 'edit' && !isRoot && (
        <FolderFormDialog
          folders={list}
          folder={current}
          defaultParentId={current.parentId ?? root.id}
          onClose={() => setDialog(null)}
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
    </div>
  )
}
