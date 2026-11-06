import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from './api'
import type { ItemFilter } from './types'

export function useMe() {
  return useQuery({
    queryKey: ['me'],
    queryFn: () =>
      api.me().catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) return null
        throw err
      }),
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: api.logout,
    onSuccess: () => {
      qc.setQueryData(['me'], null)
      qc.removeQueries({ queryKey: ['items'] })
      qc.removeQueries({ queryKey: ['item'] })
    },
  })
}

export function useItems(filter: ItemFilter) {
  return useQuery({
    queryKey: ['items', filter],
    queryFn: () => api.listItems(filter),
  })
}

export function useItem(id: string, enabled = true) {
  return useQuery({
    queryKey: ['item', id],
    queryFn: () => api.getItem(id),
    enabled: enabled && id !== '',
    retry: (count, err) => !(err instanceof ApiError && err.status === 404) && count < 2,
  })
}

export function usePublicUser(id: string) {
  return useQuery({
    queryKey: ['user', id],
    queryFn: () => api.publicUser(id),
    enabled: id !== '',
    retry: (count, err) => !(err instanceof ApiError && err.status === 404) && count < 2,
  })
}

export function useUserItems(id: string) {
  return useQuery({
    queryKey: ['user-items', id],
    queryFn: () => api.userItems(id),
    enabled: id !== '',
  })
}

export function useCollections(enabled = true) {
  return useQuery({
    queryKey: ['collections'],
    queryFn: api.collections,
    enabled,
  })
}

export function usePopularTags() {
  return useQuery({
    queryKey: ['popular-tags'],
    queryFn: () => api.popularTags(),
    staleTime: 5 * 60_000,
  })
}

export function useTags(enabled = true) {
  return useQuery({
    queryKey: ['tags'],
    queryFn: api.tags,
    enabled,
  })
}
