import type { MaybeRefOrGetter } from 'vue'
import type { Facets } from '~/types/api'

const emptyFacets = (): Facets => ({ eventNames: [], aggregateNames: [] })

export function useStoreFacets(storeID: MaybeRefOrGetter<string>) {
  const cache = useState<Record<string, Facets>>('stores:facets', () => ({}))
  const pending = ref(false)
  const error = ref('')
  const { request } = useEventStoreAPI()
  const resolvedStoreID = computed(() => toValue(storeID))
  const facets = computed(() => cache.value[resolvedStoreID.value] || emptyFacets())

  async function load(force = false): Promise<Facets> {
    const id = resolvedStoreID.value
    if (!id) return emptyFacets()
    if (cache.value[id] && !force) return cache.value[id]
    pending.value = true
    error.value = ''
    try {
      const response = await request<Facets>(`/api/v1/stores/${encodeURIComponent(id)}/facets`)
      cache.value = { ...cache.value, [id]: response }
      return response
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : 'Could not load store facets.'
      throw cause
    } finally {
      pending.value = false
    }
  }

  return { facets, pending, error, load }
}
