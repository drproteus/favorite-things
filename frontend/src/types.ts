export type Visibility = 'public' | 'private'

export interface User {
  id: string
  email: string
  name: string
  avatarUrl: string
  isAdmin: boolean
}

export interface ItemImage {
  id: string
  url: string
  position: number
}

export interface Collection {
  id: string
  parentId: string | null
  name: string
  visibility: Visibility
  createdAt: string
  updatedAt: string
}

export interface CollectionInput {
  name: string
  visibility: Visibility
  parentId: string
  // Also apply the visibility to every subfolder and item inside the folder.
  recursive?: boolean
}

export type DeleteContents = 'delete' | 'move'

export interface Item {
  id: string
  ownerId: string
  ownerName: string
  title: string
  description: string
  collectionId: string
  visibility: Visibility
  tags: string[]
  images: ItemImage[]
  rating: number | null
  createdAt: string
  updatedAt: string
}

export interface Suggestion {
  title: string
  description: string
  tags: string[]
}

export interface ItemInput {
  title: string
  description: string
  collectionId: string
  visibility: Visibility
}

export interface ItemFilter {
  scope: 'mine' | 'public'
  q?: string
  tag?: string
  collectionId?: string
  visibility?: Visibility | ''
}

export function itemToInput(item: Item): ItemInput {
  return {
    title: item.title,
    description: item.description,
    collectionId: item.collectionId,
    visibility: item.visibility,
  }
}
