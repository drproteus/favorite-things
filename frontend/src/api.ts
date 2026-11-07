import type {
  Collection,
  CollectionInput,
  DeleteContents,
  Item,
  ItemComment,
  ItemCommentInput,
  ItemFilter,
  ItemImage,
  ItemInput,
  PopularTag,
  PublicUser,
  Suggestion,
  User,
  UserSettingsInput,
  Visibility,
} from './types'

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

type RequestOptions = Omit<RequestInit, 'body'> & { body?: BodyInit; json?: unknown }

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { json, headers, body, ...rest } = options
  const finalHeaders: Record<string, string> = { ...(headers as Record<string, string>) }
  let payload: BodyInit | undefined = body
  if (json !== undefined) {
    finalHeaders['Content-Type'] = 'application/json'
    payload = JSON.stringify(json)
  }

  const res = await fetch(`/api${path}`, {
    ...rest,
    headers: finalHeaders,
    body: payload,
    credentials: 'same-origin',
  })
  if (res.status === 204) return undefined as T

  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new ApiError(res.status, data.error ?? res.statusText)
  }
  return data as T
}

export const api = {
  providers: () => request<{ providers: string[] }>('/auth/providers'),
  me: () => request<User>('/me'),
  updateMe: (input: UserSettingsInput) => request<User>('/me', { method: 'PUT', json: input }),
  publicUser: (id: string) => request<PublicUser>(`/users/${id}`),
  publicCollections: (id: string) => request<Collection[]>(`/users/${id}/collections`),
  logout: () => request<void>('/auth/logout', { method: 'POST' }),

  listItems: (f: ItemFilter) => {
    const params = new URLSearchParams({ scope: f.scope })
    if (f.q) params.set('q', f.q)
    if (f.tag) params.set('tag', f.tag)
    if (f.owner) params.set('owner', f.owner)
    if (f.collectionId) params.set('collection', f.collectionId)
    if (f.visibility) params.set('visibility', f.visibility)
    return request<Item[]>(`/items?${params}`)
  },
  getItem: (id: string) => request<Item>(`/items/${id}`),
  createItem: (input: ItemInput) => request<Item>('/items', { method: 'POST', json: input }),
  updateItem: (id: string, input: ItemInput) =>
    request<Item>(`/items/${id}`, { method: 'PUT', json: input }),
  deleteItem: (id: string) => request<void>(`/items/${id}`, { method: 'DELETE' }),

  uploadImage: (itemId: string, file: File) => {
    const form = new FormData()
    form.append('file', file)
    return request<ItemImage>(`/items/${itemId}/images`, { method: 'POST', body: form })
  },
  suggestItem: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return request<Suggestion>('/items/suggest', { method: 'POST', body: form })
  },
  deleteImage: (itemId: string, imageId: string) =>
    request<void>(`/items/${itemId}/images/${imageId}`, { method: 'DELETE' }),

  setRating: (itemId: string, rating: number) =>
    request<Item>(`/items/${itemId}/rating`, { method: 'PUT', json: { rating } }),
  clearRating: (itemId: string) => request<void>(`/items/${itemId}/rating`, { method: 'DELETE' }),

  listComments: (itemId: string) => request<ItemComment[]>(`/items/${itemId}/comments`),
  createComment: (itemId: string, input: ItemCommentInput) =>
    request<ItemComment>(`/items/${itemId}/comments`, { method: 'POST', json: input }),
  updateComment: (itemId: string, commentId: string, input: ItemCommentInput) =>
    request<ItemComment>(`/items/${itemId}/comments/${commentId}`, { method: 'PUT', json: input }),

  popularTags: (limit = 12) => request<PopularTag[]>(`/tags/popular?limit=${limit}`),

  collections: () => request<Collection[]>('/collections'),
  createCollection: (input: CollectionInput) =>
    request<Collection>('/collections', { method: 'POST', json: input }),
  updateCollection: (id: string, input: CollectionInput) =>
    request<Collection>(`/collections/${id}`, { method: 'PUT', json: input }),
  uploadFolderBanner: (id: string, file: File) => {
    const form = new FormData()
    form.append('file', file)
    return request<Collection>(`/collections/${id}/banner`, { method: 'POST', body: form })
  },
  deleteFolderBanner: (id: string) => request<void>(`/collections/${id}/banner`, { method: 'DELETE' }),
  deleteCollection: (id: string, contents: DeleteContents) =>
    request<void>(`/collections/${id}?contents=${contents}`, { method: 'DELETE' }),

  bulkMoveItems: (ids: string[], collectionId: string) =>
    request<void>('/items/bulk/move', { method: 'POST', json: { ids, collectionId } }),
  bulkDeleteItems: (ids: string[]) => request<void>('/items/bulk/delete', { method: 'POST', json: { ids } }),
  bulkMoveCollections: (ids: string[], parentId: string) =>
    request<void>('/collections/bulk/move', { method: 'POST', json: { ids, parentId } }),
  bulkDeleteCollections: (ids: string[], contents: DeleteContents) =>
    request<void>(`/collections/bulk/delete?contents=${contents}`, { method: 'POST', json: { ids } }),
  bulkVisibilityItems: (ids: string[], visibility: Visibility) =>
    request<void>('/items/bulk/visibility', { method: 'POST', json: { ids, visibility } }),
  bulkVisibilityCollections: (ids: string[], visibility: Visibility, recursive: boolean) =>
    request<void>('/collections/bulk/visibility', { method: 'POST', json: { ids, visibility, recursive } }),
}
