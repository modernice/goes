<script setup lang="ts">
import { Inbox, TriangleAlert } from '@lucide/vue'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { StoredEvent } from '~/types/api'
import { formatDate, formatDayTime, payloadPreview } from '~/utils/format'

withDefaults(defineProps<{
  events: StoredEvent[]
  loading?: boolean
  compact?: boolean
  emptyTitle?: string
  emptyHint?: string
}>(), {
  emptyTitle: 'No events found',
  emptyHint: 'Events appear here as soon as they are written to the store.',
})

const emit = defineEmits<{ select: [event: StoredEvent] }>()
</script>

<template>
  <div v-if="loading && !events.length" class="space-y-1.5 p-3">
    <Skeleton v-for="index in 8" :key="index" class="h-9" />
  </div>

  <Table v-else-if="events.length" :class="compact ? 'min-w-[520px]' : 'min-w-[760px]'">
    <TableHeader>
      <TableRow class="hover:bg-transparent">
        <TableHead class="h-9 pl-4 font-mono text-[11px] font-medium tracking-[0.05em] uppercase text-muted-foreground">Event</TableHead>
        <TableHead class="h-9 font-mono text-[11px] font-medium tracking-[0.05em] uppercase text-muted-foreground">Aggregate</TableHead>
        <TableHead v-if="!compact" class="h-9 w-2/5 font-mono text-[11px] font-medium tracking-[0.05em] uppercase text-muted-foreground">Payload</TableHead>
        <TableHead class="h-9 pr-4 text-right font-mono text-[11px] font-medium tracking-[0.05em] uppercase text-muted-foreground">Time</TableHead>
      </TableRow>
    </TableHeader>
    <TableBody>
      <TableRow
        v-for="event in events"
        :key="event.id"
        class="group cursor-pointer border-border/60"
        tabindex="0"
        :title="event.id"
        @click="emit('select', event)"
        @keydown.enter="emit('select', event)"
      >
        <TableCell class="h-11 max-w-64 pl-4">
          <span class="block truncate font-mono text-xs font-medium">{{ event.name }}</span>
        </TableCell>
        <TableCell class="h-11 max-w-56">
          <div v-if="event.aggregate" class="min-w-0" :title="`${event.aggregate.name} · ${event.aggregate.id}`">
            <div class="flex min-w-0 items-baseline gap-1.5">
              <span class="truncate text-xs">{{ event.aggregate.name }}</span>
              <span class="shrink-0 font-mono text-[11px] text-muted-foreground">v{{ event.aggregate.version }}</span>
            </div>
            <span class="mt-0.5 block truncate font-mono text-[10px] text-muted-foreground/70">{{ event.aggregate.id }}</span>
          </div>
          <span v-else class="text-xs text-muted-foreground/60">—</span>
        </TableCell>
        <TableCell v-if="!compact" class="h-11 max-w-96">
          <span v-if="event.dataDecodeError" class="flex min-w-0 items-center gap-1.5 font-mono text-[11px] text-warning">
            <TriangleAlert :size="13" class="shrink-0" />
            <span class="truncate">{{ event.dataDecodeError }}</span>
          </span>
          <span v-else class="block truncate font-mono text-[11px] text-muted-foreground">{{ payloadPreview(event.data) }}</span>
        </TableCell>
        <TableCell class="h-11 pr-4 text-right">
          <time class="font-mono text-[11px] whitespace-nowrap text-muted-foreground" :title="formatDate(event.time)">{{ formatDayTime(event.time) }}</time>
        </TableCell>
      </TableRow>
    </TableBody>
  </Table>

  <EmptyState v-else :icon="Inbox" :title="emptyTitle" :hint="emptyHint" />
</template>
