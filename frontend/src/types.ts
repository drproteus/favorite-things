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

export interface Item {
  id: string
  ownerId: string
  ownerName: string
  title: string
  description: string
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
  visibility: Visibility
}

export interface ItemFilter {
  scope: 'mine' | 'public'
  q?: string
  tag?: string
  visibility?: Visibility | ''
}

export function itemToInput(item: Item): ItemInput {
  return {
    title: item.title,
    description: item.description,
    visibility: item.visibility,
  }
}
