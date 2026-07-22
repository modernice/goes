<script setup lang="ts">
import { GitBranch, LoaderCircle, RefreshCw, Search } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { StoredEvent, Stream } from '~/types/api'
import { formatDate, formatExactNumber } from '~/utils/format'
import { routeQueryStrings } from '~/utils/query'

definePageMeta({ title: 'Streams' })
useHead({ title: 'Streams · goes Event Store' })

const route = useRoute()
const { selectedStoreID } = useStores()
const facetsState = useStoreFacets(selectedStoreID)
const explorer = useStreamsExplorer(selectedStoreID)
const selectedStream = ref<Stream | null>(null)
const selectedEvent = ref<StoredEvent | null>(null)
const loading = computed(() => explorer.status.value === 'pending')

function applyRouteFilters(): void {
  explorer.filters.aggregateNames = routeQueryStrings(route.query.aggregateName)
  explorer.filters.aggregateId = typeof route.query.aggregateId === 'string' ? route.query.aggregateId : ''
}

onMounted(() => {
  applyRouteFilters()
  if (selectedStoreID.value) {
    void facetsState.load().catch(() => undefined)
    void explorer.refresh()
  }
})

watch(selectedStoreID, (storeID) => {
  selectedStream.value = null
  if (storeID) void facetsState.load().catch(() => undefined)
})

watch(() => explorer.items.value, (items) => {
  if (route.query.open !== '1' || !items.length) return

  selectedStream.value = items[0]!
  const { open: _open, ...query } = route.query
  void navigateTo({ path: route.path, query }, { replace: true })
}, { deep: false })

async function applyFilters(): Promise<void> {
  selectedStream.value = null
  await navigateTo({
    path: '/streams',
    query: {
      ...(explorer.filters.aggregateNames.length && { aggregateName: explorer.filters.aggregateNames }),
      ...(explorer.filters.aggregateId && { aggregateId: explorer.filters.aggregateId }),
    },
    replace: true,
  })
  await explorer.refresh()
}

async function clearFilters(): Promise<void> {
  selectedStream.value = null
  await navigateTo('/streams', { replace: true })
  await explorer.clearFilters()
}

function inspectEvent(event: StoredEvent): void {
  selectedEvent.value = event
}
</script>

<template>
  <section>
    <header class="mb-5 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 class="font-mono text-[15px] font-bold tracking-tight">Streams</h1>
        <p class="mt-0.5 text-[13px] text-muted-foreground">Aggregate streams discovered from their first event.</p>
      </div>
      <Button variant="outline" size="sm" :disabled="loading" @click="explorer.refresh()">
        <RefreshCw :class="{ 'animate-spin': loading }" /> Refresh
      </Button>
    </header>

    <form
      class="mb-4 grid items-end gap-3 rounded-xl bg-card p-3.5 ring-1 ring-foreground/10 md:grid-cols-[1fr_1.4fr_auto]"
      @submit.prevent="applyFilters"
    >
      <div class="grid gap-1.5">
        <Label for="stream-aggregate-types" class="text-[11px] text-muted-foreground">Aggregate type</Label>
        <FacetMultiSelect
          id="stream-aggregate-types"
          v-model="explorer.filters.aggregateNames"
          :facets="facetsState.facets.value.aggregateNames"
          all-label="All aggregate types"
          selected-label="aggregate types"
        />
      </div>
      <div class="grid gap-1.5">
        <Label for="stream-aggregate-id" class="text-[11px] text-muted-foreground">Aggregate ID</Label>
        <Input id="stream-aggregate-id" v-model="explorer.filters.aggregateId" class="font-mono text-xs" placeholder="Exact UUID" />
      </div>
      <div class="flex gap-2">
        <Button type="submit"><Search /> Find</Button>
        <Button v-if="explorer.hasFilters.value" type="button" variant="ghost" @click="clearFilters">Reset</Button>
      </div>
    </form>

    <div v-if="explorer.error.value" class="mb-4 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-[13px] text-destructive">
      {{ explorer.error.value.message }}
    </div>

    <div class="overflow-hidden rounded-xl bg-card ring-1 ring-foreground/10">
      <div class="flex h-10 items-center border-b px-4">
        <p class="text-xs text-muted-foreground">
          <span class="font-medium text-foreground tabular-nums">{{ formatExactNumber(explorer.items.value.length) }}</span>
          streams loaded<template v-if="explorer.hasFilters.value"> · filtered</template>
        </p>
      </div>

      <div v-if="loading && !explorer.items.value.length" class="space-y-1.5 p-3">
        <Skeleton v-for="index in 8" :key="index" class="h-11" />
      </div>

      <div v-else-if="explorer.items.value.length" class="overflow-x-auto">
        <Table class="min-w-[680px]">
          <TableHeader>
            <TableRow class="hover:bg-transparent">
              <TableHead class="h-9 pl-4 font-mono text-[11px] font-medium tracking-[0.05em] uppercase text-muted-foreground">Aggregate</TableHead>
              <TableHead class="h-9 font-mono text-[11px] font-medium tracking-[0.05em] uppercase text-muted-foreground">Aggregate ID</TableHead>
              <TableHead class="h-9 pr-4 text-right font-mono text-[11px] font-medium tracking-[0.05em] uppercase text-muted-foreground">Created</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow
              v-for="stream in explorer.items.value"
              :key="`${stream.aggregateName}:${stream.aggregateId}`"
              class="cursor-pointer border-border/60"
              tabindex="0"
              @click="selectedStream = stream"
              @keydown.enter="selectedStream = stream"
            >
              <TableCell class="h-11 pl-4">
                <span class="flex items-center gap-2 font-mono text-xs font-medium">
                  <GitBranch :size="13" class="text-muted-foreground" /> {{ stream.aggregateName }}
                </span>
              </TableCell>
              <TableCell class="h-11 max-w-80">
                <code class="block truncate font-mono text-[11px] text-muted-foreground" :title="stream.aggregateId">{{ stream.aggregateId }}</code>
              </TableCell>
              <TableCell class="h-11 pr-4 text-right">
                <time class="font-mono text-[11px] whitespace-nowrap text-muted-foreground" :title="formatDate(stream.createdAt)">{{ formatDate(stream.createdAt) }}</time>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <EmptyState v-else :icon="GitBranch" title="No matching streams" hint="Streams appear after their version-1 event is discovered." />

      <Button
        v-if="explorer.nextCursor.value"
        variant="ghost"
        class="h-10 w-full rounded-none border-t text-xs text-muted-foreground"
        :disabled="explorer.loadingMore.value"
        @click="explorer.loadMore()"
      >
        <LoaderCircle v-if="explorer.loadingMore.value" class="animate-spin" />
        {{ explorer.loadingMore.value ? 'Loading…' : 'Load older streams' }}
      </Button>
    </div>

    <StreamDialog :store-id="selectedStoreID" :stream="selectedStream" @close="selectedStream = null" @select-event="inspectEvent" />
    <EventDialog :event="selectedEvent" @close="selectedEvent = null" @open-stream="selectedEvent = null" />
  </section>
</template>
