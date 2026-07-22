import type { StoreInfo } from '~/types/api'

export function useStores() {
  const stores = useState<StoreInfo[]>('stores:items', () => [])
  const selectedStoreID = useState('stores:selected', () => '')
  const pending = useState('stores:pending', () => false)
  const error = useState('stores:error', () => '')
  const { request } = useEventStoreAPI()

  const currentStore = computed(() => stores.value.find(store => store.id === selectedStoreID.value) || null)
  const onlineCount = computed(() => stores.value.filter(store => store.status === 'online').length)

  async function load(force = false): Promise<void> {
    if (stores.value.length && !force) return
    pending.value = true
    error.value = ''
    try {
      const response = await request<{ items: StoreInfo[] }>('/api/v1/stores')
      stores.value = response.items
      const saved = import.meta.client ? localStorage.getItem('goes-ui-store') : null
      const preferred = stores.value.find(store => store.id === selectedStoreID.value)
        || stores.value.find(store => store.id === saved)
        || stores.value.find(store => store.status === 'online')
        || stores.value[0]
      select(preferred?.id || '')
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : 'Could not load event stores.'
      throw cause
    } finally {
      pending.value = false
    }
  }

  function select(storeID: string): void {
    selectedStoreID.value = storeID
    if (import.meta.client && storeID) localStorage.setItem('goes-ui-store', storeID)
  }

  function reset(): void {
    stores.value = []
    selectedStoreID.value = ''
    error.value = ''
  }

  return { stores, selectedStoreID, currentStore, onlineCount, pending, error, load, select, reset }
}
