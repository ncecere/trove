import {
  keepPreviousData,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { api } from './client'
import type {
  ItemQuery,
  Library,
  LibraryCreate,
  LibraryUpdate,
  SettingsUpdate,
  User,
  UserCreate,
  UserUpdate,
} from './types'

export const qk = {
  setup: ['setup'] as const,
  me: ['me'] as const,
  libraries: ['libraries'] as const,
  home: ['home'] as const,
  items: (q: ItemQuery) => ['items', q] as const,
  itemsAll: ['items'] as const,
  item: (id: number) => ['item', id] as const,
  genres: (lib?: number) => ['genres', lib ?? null] as const,
  users: ['users'] as const,
  fs: (path: string) => ['fs', path] as const,
  settings: ['settings'] as const,
  activity: (page: number, limit: number) => ['activity', page, limit] as const,
}

const anyScanning = (libs: Library[] | undefined) => !!libs?.some((l) => l.scan.state === 'scanning')

// ---- session ----

export function useSetupStatus() {
  return useQuery({ queryKey: qk.setup, queryFn: ({ signal }) => api.setupStatus(signal), staleTime: Infinity })
}

export function useMe() {
  return useQuery({ queryKey: qk.me, queryFn: ({ signal }) => api.me(signal), staleTime: 5 * 60_000 })
}

/** The signed-in user. Only call below the auth guard. */
export function useUser(): User {
  const { data } = useMe()
  if (!data) throw new Error('useUser() called without a signed-in user')
  return data
}

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: api.login,
    onSuccess: (user) => {
      qc.removeQueries({ predicate: (q) => q.queryKey[0] !== 'me' && q.queryKey[0] !== 'setup' })
      qc.setQueryData(qk.me, user)
    },
  })
}

export function useSetup() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: api.setup,
    onSuccess: (user) => {
      qc.setQueryData(qk.setup, { needsSetup: false })
      qc.setQueryData(qk.me, user)
    },
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: api.logout,
    onSettled: () => {
      qc.setQueryData(qk.me, null)
      qc.removeQueries({ predicate: (q) => q.queryKey[0] !== 'me' && q.queryKey[0] !== 'setup' })
    },
  })
}

export function useChangePassword() {
  return useMutation({
    mutationFn: (v: { currentPassword: string; newPassword: string }) =>
      api.changePassword(v.currentPassword, v.newPassword),
  })
}

// ---- browsing ----

export function useLibraries() {
  return useQuery({
    queryKey: qk.libraries,
    queryFn: ({ signal }) => api.libraries(signal),
    refetchInterval: (q) => (anyScanning(q.state.data) ? 2000 : false),
  })
}

export function useHome() {
  return useQuery({ queryKey: qk.home, queryFn: ({ signal }) => api.home(signal) })
}

export const PAGE_SIZE = 48

export function useItemsInfinite(q: Omit<ItemQuery, 'page' | 'limit'>) {
  return useInfiniteQuery({
    queryKey: qk.items(q),
    queryFn: ({ pageParam, signal }) => api.items({ ...q, page: pageParam, limit: PAGE_SIZE }, signal),
    initialPageParam: 1,
    getNextPageParam: (last) => (last.page * last.limit < last.total ? last.page + 1 : undefined),
    placeholderData: keepPreviousData,
  })
}

export function useItems(q: ItemQuery, enabled = true) {
  return useQuery({
    queryKey: qk.items(q),
    queryFn: ({ signal }) => api.items(q, signal),
    enabled,
    placeholderData: keepPreviousData,
  })
}

export function useItem(id: number) {
  return useQuery({ queryKey: qk.item(id), queryFn: ({ signal }) => api.item(id, signal), enabled: id > 0 })
}

export function useGenres(library?: number) {
  return useQuery({ queryKey: qk.genres(library), queryFn: ({ signal }) => api.genres(library, signal) })
}

export function useRefreshItem() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: api.refreshItem,
    onSuccess: (item) => {
      qc.setQueryData(qk.item(item.id), item)
      // Metadata is fetched in the background; pick up the result a little later.
      window.setTimeout(() => void qc.invalidateQueries({ queryKey: qk.item(item.id) }), 4000)
    },
  })
}

// ---- admin ----

function useInvalidateLibraryData() {
  const qc = useQueryClient()
  return () => {
    void qc.invalidateQueries({ queryKey: qk.libraries })
    void qc.invalidateQueries({ queryKey: qk.home })
    void qc.invalidateQueries({ queryKey: qk.itemsAll })
    void qc.invalidateQueries({ queryKey: ['genres'] })
  }
}

export function useCreateLibrary() {
  const inv = useInvalidateLibraryData()
  return useMutation({ mutationFn: (b: LibraryCreate) => api.createLibrary(b), onSuccess: inv })
}

export function useUpdateLibrary() {
  const inv = useInvalidateLibraryData()
  return useMutation({
    mutationFn: (v: { id: number; body: LibraryUpdate }) => api.updateLibrary(v.id, v.body),
    onSuccess: inv,
  })
}

export function useDeleteLibrary() {
  const inv = useInvalidateLibraryData()
  return useMutation({ mutationFn: (id: number) => api.deleteLibrary(id), onSuccess: inv })
}

export function useScanLibrary() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.scanLibrary(id),
    onSuccess: (lib) => {
      qc.setQueryData<Library[]>(qk.libraries, (old) => old?.map((l) => (l.id === lib.id ? lib : l)))
      void qc.invalidateQueries({ queryKey: qk.libraries })
    },
  })
}

export function useScanAll() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.scanAll(),
    onSuccess: (libs) => qc.setQueryData(qk.libraries, libs),
  })
}

export function useFs(path: string, enabled: boolean) {
  return useQuery({
    queryKey: qk.fs(path),
    queryFn: ({ signal }) => api.fs(path, signal),
    enabled,
    placeholderData: keepPreviousData,
  })
}

export function useUsers() {
  return useQuery({ queryKey: qk.users, queryFn: ({ signal }) => api.users(signal) })
}

export function useCreateUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (b: UserCreate) => api.createUser(b),
    onSuccess: () => void qc.invalidateQueries({ queryKey: qk.users }),
  })
}

export function useUpdateUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (v: { id: number; body: UserUpdate }) => api.updateUser(v.id, v.body),
    onSuccess: (user) => {
      void qc.invalidateQueries({ queryKey: qk.users })
      qc.setQueryData<User | null>(qk.me, (me) => (me && me.id === user.id ? user : me))
    },
  })
}

export function useDeleteUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.deleteUser(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey: qk.users }),
  })
}

export function useSettings() {
  return useQuery({ queryKey: qk.settings, queryFn: ({ signal }) => api.settings(signal) })
}

export function useUpdateSettings() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (b: SettingsUpdate) => api.updateSettings(b),
    onSuccess: (s) => qc.setQueryData(qk.settings, s),
  })
}

export function useActivity(page: number, limit: number) {
  return useQuery({
    queryKey: qk.activity(page, limit),
    queryFn: ({ signal }) => api.activity(page, limit, signal),
    placeholderData: keepPreviousData,
  })
}
