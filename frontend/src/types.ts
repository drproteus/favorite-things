export type Visibility = 'public' | 'private'

export interface User {
  id: string
  email: string
  name: string
  avatarUrl: string
  isAdmin: boolean
  profile: string
  defaultVisibility: Visibility
}

// What anyone can see about a user.
export interface PublicUser {
  id: string
  name: string
  avatarUrl: string
  profile: string
}

export interface UserSettingsInput {
  profile: string
  defaultVisibility: Visibility
}

export interface ImageVariant {
  url: string
  width: number
  height: number
}

// The fields needed to render a responsive image. Collections use the same shape
// for their banner through bannerUrl/bannerVariants.
export interface ImageSource {
  url: string
  width?: number
  height?: number
  variants?: ImageVariant[]
}

export interface ItemImage extends ImageSource {
  id: string
  position: number
}

export interface Collection {
  id: string
  parentId: string | null
  name: string
  visibility: Visibility
  description: string
  bannerUrl: string | null
  bannerWidth?: number
  bannerHeight?: number
  bannerVariants?: ImageVariant[]
  createdAt: string
  updatedAt: string
}

export interface CollectionInput {
  name: string
  visibility: Visibility
  parentId: string
  description?: string
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

export interface PopularTag {
  tag: string
  count: number
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
  // Public listings only: one user's public items.
  owner?: string
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
