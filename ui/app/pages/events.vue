<script setup lang="ts">
import { LoaderCircle, RefreshCw } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import type { StoredEvent } from '~/types/api'
import { formatExactNumber } from '~/utils/format'
import { routeQueryStrings } from '~/utils/query'

definePageMeta({ title: 'Events' })
useHead({ title: 'Events · goes Event Store' })

const route = useRoute()
const { selectedStoreID } = useStores()
const facetsState = useStoreFacets(selectedStoreID)
const explorer = useEventsExplorer(selectedStoreID)
const selectedEvent = ref<StoredEvent | null>(null)
const loading = computed(() => explorer.status.value === 'pending')

const { enabled: liveRefresh } = useLiveRefresh(() => explorer.refresh())

function applyRouteFilters(): void {
  explorer.filters.names = routeQueryStrings(route.query.name)
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

watch(selectedStoreID, storeID => { if (storeID) void facetsState.load().catch(() => undefined) })

async function applyFilters(): Promise<void> {
  await navigateTo({
    path: '/events',
    query: {
      ...(explorer.filters.names.length && { name: explorer.filters.names }),
      ...(explorer.filters.aggregateNames.length && { aggregateName: explorer.filters.aggregateNames }),
      ...(explorer.filters.aggregateId && { aggregateId: explorer.filters.aggregateId }),
    },
    replace: true,
  })
  await explorer.refresh()
}

async function clearFilters(): Promise<void> {
  await navigateTo('/events', { replace: true })
  await explorer.clearFilters()
}

async function openStream(event: StoredEvent): Promise<void> {
  if (!event.aggregate) return
  selectedEvent.value = null
  await navigateTo({ path: '/streams', query: { aggregateName: event.aggregate.name, aggregateId: event.aggregate.id, open: '1' } })
}

</script>

<template>
  <section>
    <header class="mb-5 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 class="font-mono text-[15px] font-bold tracking-tight">Events</h1>
        <p class="mt-0.5 text-[13px] text-muted-foreground">The append-only timeline, newest first.</p>
      </div>
      <div class="flex items-center gap-2">
        <label class="flex h-7 cursor-pointer items-center gap-2 rounded-lg border px-2.5 text-xs select-none">
          <span class="relative flex size-1.5">
            <span v-if="liveRefresh" class="absolute inline-flex size-full animate-ping rounded-full bg-brand opacity-60" />
            <span :class="['relative inline-flex size-1.5 rounded-full transition-colors', liveRefresh ? 'bg-brand' : 'bg-muted-foreground/40']" />
          </span>
          Live
          <Switch v-model="liveRefresh" size="sm" aria-label="Refresh automatically" />
        </label>
        <Button variant="outline" size="sm" :disabled="loading" @click="explorer.refresh()">
          <RefreshCw :class="{ 'animate-spin': loading }" /> Refresh
        </Button>
      </div>
    </header>

    <form
      class="mb-4 grid items-end gap-3 rounded-xl bg-card p-3.5 ring-1 ring-foreground/10 md:grid-cols-2 xl:grid-cols-[1.1fr_1.1fr_1.3fr_1fr_1fr_auto]"
      @submit.prevent="applyFilters"
    >
      <div class="grid gap-1.5">
        <Label for="event-types" class="text-[11px] text-muted-foreground">Event type</Label>
        <FacetMultiSelect
          id="event-types"
          v-model="explorer.filters.names"
          :facets="facetsState.facets.value.eventNames"
          all-label="All event types"
          selected-label="event types"
        />
      </div>
      <div class="grid gap-1.5">
        <Label for="event-aggregate-types" class="text-[11px] text-muted-foreground">Aggregate type</Label>
        <FacetMultiSelect
          id="event-aggregate-types"
          v-model="explorer.filters.aggregateNames"
          :facets="facetsState.facets.value.aggregateNames"
          all-label="All aggregate types"
          selected-label="aggregate types"
        />
      </div>
      <div class="grid gap-1.5">
        <Label for="aggregate-id" class="text-[11px] text-muted-foreground">Aggregate ID</Label>
        <Input id="aggregate-id" v-model="explorer.filters.aggregateId" class="font-mono text-xs" placeholder="Exact UUID" />
      </div>
      <div class="grid gap-1.5">
        <Label for="from-time" class="text-[11px] text-muted-foreground">From</Label>
        <Input id="from-time" v-model="explorer.filters.from" type="datetime-local" class="text-xs" />
      </div>
      <div class="grid gap-1.5">
        <Label for="to-time" class="text-[11px] text-muted-foreground">To</Label>
        <Input id="to-time" v-model="explorer.filters.to" type="datetime-local" class="text-xs" />
      </div>
      <div class="flex gap-2">
        <Button type="submit" size="default">Apply</Button>
        <Button v-if="explorer.hasFilters.value" type="button" variant="ghost" @click="clearFilters">Reset</Button>
      </div>
    </form>

    <div v-if="explorer.error.value" class="mb-4 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-[13px] text-destructive">
      {{ explorer.error.value.message }}
    </div>

    <div class="overflow-hidden rounded-xl bg-card ring-1 ring-foreground/10">
      <div class="flex h-10 items-center justify-between gap-3 border-b px-4">
        <p class="text-xs text-muted-foreground">
          <span class="font-medium text-foreground tabular-nums">{{ formatExactNumber(explorer.items.value.length) }}</span>
          events loaded<template v-if="explorer.hasFilters.value"> · filtered</template>
        </p>
        <p v-if="liveRefresh" class="flex items-center gap-1.5 font-mono text-[11px] text-brand">
          <span class="size-1 rounded-full bg-brand" /> refreshing every 5s
        </p>
      </div>
      <div class="overflow-x-auto">
        <EventTable
          :events="explorer.items.value"
          :loading="loading"
          empty-hint="No events match the current filters."
          @select="selectedEvent = $event"
        />
      </div>
      <Button
        v-if="explorer.nextCursor.value"
        variant="ghost"
        class="h-10 w-full rounded-none border-t text-xs text-muted-foreground"
        :disabled="explorer.loadingMore.value"
        @click="explorer.loadMore()"
      >
        <LoaderCircle v-if="explorer.loadingMore.value" class="animate-spin" />
        {{ explorer.loadingMore.value ? 'Loading…' : 'Load older events' }}
      </Button>
    </div>

    <EventDialog :event="selectedEvent" @close="selectedEvent = null" @open-stream="openStream" />
  </section>
</template>
