import type { Collection } from './types'

export function rootFolder(list: Collection[]): Collection | undefined {
  return list.find((c) => c.parentId === null)
}

export function childrenOf(list: Collection[], parentId: string): Collection[] {
  return list.filter((c) => c.parentId === parentId)
}

// The folder followed by its ancestors, from the root down to the folder itself.
export function pathTo(list: Collection[], id: string): Collection[] {
  const path: Collection[] = []
  let current = list.find((c) => c.id === id)
  while (current) {
    path.unshift(current)
    current = current.parentId ? list.find((c) => c.id === current!.parentId) : undefined
  }
  return path
}

// Every folder in tree order with its depth. Passing excludeId skips that folder and everything under it.
export function flattenFolders(list: Collection[], excludeId?: string): { folder: Collection; depth: number }[] {
  const root = rootFolder(list)
  if (!root) return []
  const out: { folder: Collection; depth: number }[] = []
  const walk = (folder: Collection, depth: number) => {
    out.push({ folder, depth })
    for (const child of childrenOf(list, folder.id)) {
      if (child.id !== excludeId) walk(child, depth + 1)
    }
  }
  walk(root, 0)
  return out
}
