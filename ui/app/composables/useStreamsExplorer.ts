import type { MaybeRefOrGetter } from 'vue'
import type { Page, Stream } from '~/types/api'

export function useStreamsExplorer(storeID: MaybeRefOrGetter<string>) {
  const { request } = useEventStoreAPI()
  const resolvedStoreID = computed(() => toValue(storeID))
  const filters = reactive({ aggregateNames: [] as string[], aggregateId: '' })

  function query(cursor = ''): URLSearchParams {
    const params = new URLSearchParams({ limit: '50' })
    filters.aggregateNames.forEach(name => params.append('aggregateName', name))
    if (filters.aggregateId) params.set('aggregateId', filters.aggregateId)
    if (cursor) params.set('cursor', cursor)
    return params
  }

  async function fetchPage(cursor = ''): Promise<Page<Stream>> {
    const id = resolvedStoreID.value
    if (!id) return { items: [] }
    return await request<Page<Stream>>(
      `/api/v1/stores/${encodeURIComponent(id)}/streams?${query(cursor)}`,
    )
  }

  const state = useAsyncData<Page<Stream>>(
    'explorer:streams',
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
  const hasFilters = computed(() => filters.aggregateNames.length > 0 || !!filters.aggregateId)

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
    Object.assign(filters, { aggregateNames: [], aggregateId: '' })
    await state.refresh()
  }

  return { ...state, filters, items, nextCursor, hasFilters, loadingMore, loadMore, clearFilters }
}
