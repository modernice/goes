import type { MaybeRefOrGetter } from 'vue'
import type { Page, StoredEvent } from '~/types/api'

export interface EventFilters {
  names: string[]
  aggregateNames: string[]
  aggregateId: string
  from: string
  to: string
}

export function useEventsExplorer(storeID: MaybeRefOrGetter<string>) {
  const { request } = useEventStoreAPI()
  const resolvedStoreID = computed(() => toValue(storeID))
  const filters = reactive<EventFilters>({ names: [], aggregateNames: [], aggregateId: '', from: '', to: '' })

  function query(cursor = ''): URLSearchParams {
    const params = new URLSearchParams({ limit: '50' })
    filters.names.forEach(name => params.append('name', name))
    filters.aggregateNames.forEach(name => params.append('aggregateName', name))
    if (filters.aggregateId) params.set('aggregateId', filters.aggregateId)
    if (filters.from) params.set('from', new Date(filters.from).toISOString())
    if (filters.to) params.set('to', new Date(filters.to).toISOString())
    if (cursor) params.set('cursor', cursor)
    return params
  }

  async function fetchPage(cursor = ''): Promise<Page<StoredEvent>> {
    const id = resolvedStoreID.value
    if (!id) return { items: [] }
    return await request<Page<StoredEvent>>(
      `/api/v1/stores/${encodeURIComponent(id)}/events?${query(cursor)}`,
    )
  }

  const state = useAsyncData<Page<StoredEvent>>(
    'explorer:events',
    () => fetchPage(),
    {
      immediate: false,
      watch: [resolvedStoreID],
      default: () => ({ items: [] }),
    },
  )

  const loadingMore = ref(false)
  const items = computed(() => state.data.value?.items || [])
  const nextCursor = computed(() => state.data.value?.nextCursor || '')
  const hasFilters = computed(() => (
    filters.names.length > 0
    || filters.aggregateNames.length > 0
    || !!filters.aggregateId
    || !!filters.from
    || !!filters.to
  ))

  async function loadMore(): Promise<void> {
    if (!nextCursor.value || loadingMore.value) return
    loadingMore.value = true
    try {
      const next = await fetchPage(nextCursor.value)
      state.data.value = { items: [...items.value, ...next.items], nextCursor: next.nextCursor }
    } finally {
      loadingMore.value = false
    }
  }

  async function clearFilters(): Promise<void> {
    Object.assign(filters, { names: [], aggregateNames: [], aggregateId: '', from: '', to: '' })
    await state.refresh()
  }

  return { ...state, filters, items, nextCursor, hasFilters, loadingMore, loadMore, clearFilters }
}
