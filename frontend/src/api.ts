import type { Collection, CollectionInput, DeleteContents, Item, ItemFilter, ItemImage, ItemInput, User } from './types'

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
  logout: () => request<void>('/auth/logout', { method: 'POST' }),

  listItems: (f: ItemFilter) => {
    const params = new URLSearchParams({ scope: f.scope })
    if (f.q) params.set('q', f.q)
    if (f.tag) params.set('tag', f.tag)
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
  deleteImage: (itemId: string, imageId: string) =>
    request<void>(`/items/${itemId}/images/${imageId}`, { method: 'DELETE' }),

  setRating: (itemId: string, rating: number) =>
    request<Item>(`/items/${itemId}/rating`, { method: 'PUT', json: { rating } }),
  clearRating: (itemId: string) => request<void>(`/items/${itemId}/rating`, { method: 'DELETE' }),

  tags: () => request<string[]>('/tags'),

  collections: () => request<Collection[]>('/collections'),
  createCollection: (input: CollectionInput) =>
    request<Collection>('/collections', { method: 'POST', json: input }),
  updateCollection: (id: string, input: CollectionInput) =>
    request<Collection>(`/collections/${id}`, { method: 'PUT', json: input }),
  deleteCollection: (id: string, contents: DeleteContents) =>
    request<void>(`/collections/${id}?contents=${contents}`, { method: 'DELETE' }),
}
