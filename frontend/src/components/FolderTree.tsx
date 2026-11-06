import { Folder, FolderOpen, Globe, Lock } from 'lucide-react'
import { childrenOf, rootFolder } from '../folders'
import type { Collection } from '../types'

interface Props {
  folders: Collection[]
  currentId: string
  onSelect: (id: string) => void
}

export default function FolderTree({ folders, currentId, onSelect }: Props) {
  const root = rootFolder(folders)
  if (!root) return null
  return (
    <nav aria-label="Folders" className="text-sm">
      <TreeNode folders={folders} folder={root} currentId={currentId} onSelect={onSelect} depth={0} />
    </nav>
  )
}

function TreeNode({
  folders,
  folder,
  currentId,
  onSelect,
  depth,
}: {
  folders: Collection[]
  folder: Collection
  currentId: string
  onSelect: (id: string) => void
  depth: number
}) {
  const active = folder.id === currentId
  const children = childrenOf(folders, folder.id)
  const Icon = active ? FolderOpen : Folder
  const name = folder.parentId === null ? 'My collection' : folder.name

  return (
    <div>
      <button
        type="button"
        onClick={() => onSelect(folder.id)}
        aria-current={active ? 'page' : undefined}
        style={{ paddingLeft: `${depth * 0.875 + 0.5}rem` }}
        className={`flex w-full cursor-pointer items-center gap-2 rounded-lg py-1.5 pr-2 text-left transition ${
          active
            ? 'bg-indigo-50 font-medium text-indigo-700 dark:bg-indigo-950/50 dark:text-indigo-300'
            : 'text-stone-600 hover:bg-stone-100 dark:text-stone-300 dark:hover:bg-stone-800'
        }`}
      >
        <Icon size={15} className="shrink-0" />
        <span className="flex-1 truncate">{name}</span>
        {folder.visibility === 'public' ? (
          <Globe size={12} className="shrink-0 text-emerald-600 dark:text-emerald-400" aria-label="Public folder" />
        ) : (
          <Lock size={12} className="shrink-0 text-stone-400" aria-label="Private folder" />
        )}
      </button>
      {children.map((child) => (
        <TreeNode
          key={child.id}
          folders={folders}
          folder={child}
          currentId={currentId}
          onSelect={onSelect}
          depth={depth + 1}
        />
      ))}
    </div>
  )
}
