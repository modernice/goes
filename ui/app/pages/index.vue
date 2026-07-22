<script setup lang="ts">
import { RefreshCw, TriangleAlert } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import type { Facet, StoredEvent } from '~/types/api'
import { formatDate, formatExactNumber, relativeTime } from '~/utils/format'

definePageMeta({ title: 'Overview' })
useHead({ title: 'Overview · goes Event Store' })

const { selectedStoreID, currentStore } = useStores()
const {
  summary,
  facets,
  recentEvents,
  summaryLoading,
  facetsLoading,
  recentLoading,
  loading,
  error,
  refresh,
} = useOverview(selectedStoreID)
const selectedEvent = ref<StoredEvent | null>(null)
const empty = computed(() => summaryLoading.value && !summary.value.totalEvents)

const stats = computed(() => [
  { label: 'Total events', value: formatExactNumber(summary.value.totalEvents) },
  { label: 'Event types', value: formatExactNumber(summary.value.eventTypes) },
  { label: 'Aggregate types', value: formatExactNumber(summary.value.aggregateTypes) },
  { label: 'Streams', value: formatExactNumber(summary.value.aggregateStreams) },
  {
    label: 'Last write',
    value: summary.value.latestEventTime ? relativeTime(summary.value.latestEventTime) : '—',
    context: summary.value.latestEventTime ? formatDate(summary.value.latestEventTime) : undefined,
  },
])

const eventTypeLink = (facet: Facet) => ({ path: '/events', query: { name: facet.value } })
const aggregateLink = (facet: Facet) => ({ path: '/streams', query: { aggregateName: facet.value } })

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
        <h1 class="font-mono text-[15px] font-bold tracking-tight">Overview</h1>
        <p class="mt-0.5 text-[13px] text-muted-foreground">Shape and activity of {{ currentStore?.name || 'the store' }}.</p>
      </div>
      <Button variant="outline" size="sm" :disabled="loading" @click="refresh()">
        <RefreshCw :class="{ 'animate-spin': loading }" /> Refresh
      </Button>
    </header>

    <div v-if="error" class="mb-4 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-[13px] text-destructive">
      {{ error.message }}
    </div>

    <div class="mb-4 overflow-hidden rounded-xl ring-1 ring-foreground/10">
      <div class="grid grid-cols-2 gap-px bg-border sm:grid-cols-3 lg:grid-cols-5">
        <div v-for="stat in stats" :key="stat.label" class="min-h-[76px] bg-card px-4 py-3">
          <p class="text-[11px] text-muted-foreground">{{ stat.label }}</p>
          <Skeleton v-if="empty" class="mt-2 h-5 w-16" />
          <template v-else>
            <p class="mt-1 truncate text-xl font-semibold tracking-tight" :title="stat.context">{{ stat.value }}</p>
            <p v-if="stat.context" class="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">{{ stat.context }}</p>
          </template>
        </div>
        <div class="bg-card lg:hidden" aria-hidden="true" />
      </div>
    </div>

    <div
      v-if="summary.payloadDecodeErrors"
      class="mb-4 flex items-center gap-2.5 rounded-lg border border-warning/30 bg-warning/8 px-4 py-2.5"
    >
      <TriangleAlert :size="14" class="shrink-0 text-warning" />
      <p class="text-xs text-warning">
        {{ formatExactNumber(summary.payloadDecodeErrors) }} payload{{ summary.payloadDecodeErrors === 1 ? '' : 's' }} could not be decoded —
        register the missing event types in your codec.
      </p>
    </div>

    <div class="grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
      <div class="overflow-hidden rounded-xl bg-card ring-1 ring-foreground/10">
        <div class="flex h-10 items-center justify-between border-b px-4">
          <h2 class="font-mono text-[13px] font-medium tracking-tight">Recent events</h2>
          <NuxtLink to="/events" class="rounded-sm text-xs text-brand outline-none hover:underline focus-visible:underline">
            View all →
          </NuxtLink>
        </div>
        <div class="overflow-x-auto">
          <EventTable
            :events="recentEvents"
            :loading="recentLoading"
            compact
            empty-title="No events yet"
            empty-hint="Events show up here as soon as they are written."
            @select="selectedEvent = $event"
          />
        </div>
      </div>

      <div class="grid gap-4">
        <FacetList
          title="Event types"
          :items="facets.eventNames"
          :loading="facetsLoading"
          :link-to="eventTypeLink"
        />
        <FacetList
          title="Aggregates"
          :items="facets.aggregateNames"
          :loading="facetsLoading"
          :link-to="aggregateLink"
        />
      </div>
    </div>

    <EventDialog :event="selectedEvent" @close="selectedEvent = null" @open-stream="openStream" />
  </section>
</template>
