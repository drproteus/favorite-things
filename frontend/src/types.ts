export type Visibility = 'public' | 'private'

export interface User {
  id: string
  email: string
  name: string
  avatarUrl: string
  isAdmin: boolean
}

export interface Term {
  id: number
  name: string
}

export interface Catalog {
  categories: Term[]
}

export type TermKind = 'categories'

export interface ItemImage {
  id: string
  url: string
  position: number
}

export interface Item {
  id: string
  ownerId: string
  ownerName: string
  title: string
  description: string
  categoryId: number
  category: string
  visibility: Visibility
  tags: string[]
  images: ItemImage[]
  rating: number | null
  createdAt: string
  updatedAt: string
}

export interface ItemInput {
  title: string
  description: string
  categoryId: number
  visibility: Visibility
}

export interface ItemFilter {
  scope: 'mine' | 'public'
  q?: string
  tag?: string
  categoryId?: number
  visibility?: Visibility | ''
}

export interface ExploreSection {
  category: Term
  count: number
  cover: ItemImage | null
  items: Item[]
}

export function itemToInput(item: Item): ItemInput {
  return {
    title: item.title,
    description: item.description,
    categoryId: item.categoryId,
    visibility: item.visibility,
  }
}
