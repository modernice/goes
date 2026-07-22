import type { MaybeRefOrGetter } from 'vue'
import type { Page, StoredEvent, Summary } from '~/types/api'

const emptySummary = (): Summary => ({
  totalEvents: 0,
  eventTypes: 0,
  aggregateTypes: 0,
  aggregateStreams: 0,
})

export function useOverview(storeID: MaybeRefOrGetter<string>) {
  const { request } = useEventStoreAPI()
  const resolvedStoreID = computed(() => toValue(storeID))
  const facetsState = useStoreFacets(resolvedStoreID)

  const summaryState = useAsyncData<Summary>(
    'overview:summary',
    async () => {
      const id = resolvedStoreID.value
      if (!id) return emptySummary()
      return await request<Summary>(`/api/v1/stores/${encodeURIComponent(id)}/summary`)
    },
    {
      immediate: false,
      default: emptySummary,
    },
  )

  const recentState = useAsyncData<Page<StoredEvent>>(
    'overview:recent-events',
    async () => {
      const id = resolvedStoreID.value
      if (!id) return { items: [] }
      return await request<Page<StoredEvent>>(
        `/api/v1/stores/${encodeURIComponent(id)}/events?limit=8`,
      )
    },
    {
      immediate: false,
      default: () => ({ items: [] }),
    },
  )

  const summary = computed(() => summaryState.data.value || emptySummary())
  const recentEvents = computed(() => recentState.data.value?.items || [])
  const summaryLoading = computed(() => summaryState.status.value === 'pending')
  const recentLoading = computed(() => recentState.status.value === 'pending')
  const facetsLoading = computed(() => facetsState.pending.value)
  const loading = computed(() => summaryLoading.value || recentLoading.value || facetsLoading.value)
  const error = computed<Error | null>(() => {
    if (summaryState.error.value) return summaryState.error.value
    if (recentState.error.value) return recentState.error.value
    return facetsState.error.value ? new Error(facetsState.error.value) : null
  })

  async function load(force = false): Promise<void> {
    if (!resolvedStoreID.value) return
    await Promise.allSettled([
      summaryState.refresh(),
      recentState.refresh(),
      facetsState.load(force),
    ])
  }

  onMounted(() => { void load() })
  watch(resolvedStoreID, (id) => { if (id) void load() })

  return {
    summary,
    facets: facetsState.facets,
    recentEvents,
    summaryLoading,
    facetsLoading,
    recentLoading,
    loading,
    error,
    refresh: () => load(true),
  }
}
