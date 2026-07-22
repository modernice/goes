<script setup lang="ts">
import { GitCommit, LoaderCircle } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import type { Page, StoredEvent, Stream } from '~/types/api'
import { formatDate, formatDayTime } from '~/utils/format'

const props = defineProps<{ storeId: string, stream: Stream | null }>()
const emit = defineEmits<{ close: [], selectEvent: [event: StoredEvent] }>()
const { request } = useEventStoreAPI()
const events = ref<StoredEvent[]>([])
const nextCursor = ref('')
const loading = ref(false)
const error = ref('')
let requestVersion = 0

async function load(append = false): Promise<void> {
  const stream = props.stream
  if (!stream || loading.value) return
  const version = ++requestVersion
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams({ limit: '50' })
    if (append && nextCursor.value) params.set('cursor', nextCursor.value)
    const path = `/api/v1/stores/${encodeURIComponent(props.storeId)}/streams/${encodeURIComponent(stream.aggregateName)}/${encodeURIComponent(stream.aggregateId)}/events?${params}`
    const page = await request<Page<StoredEvent>>(path)
    if (version !== requestVersion || props.stream !== stream) return
    events.value = append ? [...events.value, ...page.items] : page.items
    nextCursor.value = page.nextCursor || ''
  } catch (cause) {
    if (version === requestVersion) {
      error.value = cause instanceof Error ? cause.message : 'Could not load stream events.'
    }
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

watch(() => props.stream, (stream) => {
  requestVersion++
  events.value = []
  nextCursor.value = ''
  error.value = ''
  loading.value = false
  if (stream) void load()
})
</script>

<template>
  <Dialog :open="!!stream" @update:open="open => { if (!open) emit('close') }">
    <DialogContent class="flex max-h-[calc(100dvh-1rem)] max-w-2xl flex-col gap-0 overflow-hidden p-0 sm:max-h-[min(80vh,720px)]">
      <header class="shrink-0 border-b px-5 py-4 pr-14">
        <p class="text-[11px] font-medium text-muted-foreground">Aggregate stream</p>
        <DialogTitle class="mt-0.5 font-mono text-[15px] leading-snug font-semibold break-all">{{ stream?.aggregateName }}</DialogTitle>
        <div class="mt-1 flex items-center gap-1">
          <DialogDescription class="truncate font-mono text-[11px] text-muted-foreground" :title="stream?.aggregateId">{{ stream?.aggregateId }}</DialogDescription>
          <CopyButton v-if="stream" :text="stream.aggregateId" label="Copy aggregate ID" />
        </div>
        <p v-if="stream" class="mt-2 text-[11px] text-muted-foreground">
          Created {{ formatDate(stream.createdAt) }}
        </p>
      </header>

      <div class="min-h-0 flex-1 overflow-y-auto">
        <div v-if="error" class="m-4 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-[13px] text-destructive">
          {{ error }}
        </div>

        <div v-if="loading && !events.length" class="flex items-center justify-center gap-2 py-12 text-xs text-muted-foreground">
          <LoaderCircle class="size-4 animate-spin" /> Loading stream…
        </div>

        <ol v-else-if="events.length" class="divide-y">
          <li v-for="event in events" :key="event.id">
            <button
              type="button"
              class="group flex w-full items-start gap-3 px-5 py-3 text-left outline-none hover:bg-accent/60 focus-visible:bg-accent/60"
              @click="emit('selectEvent', event)"
            >
              <span class="mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full border bg-background text-muted-foreground">
                <GitCommit :size="13" />
              </span>
              <span class="min-w-0 flex-1">
                <span class="flex min-w-0 items-baseline justify-between gap-3">
                  <span class="truncate font-mono text-xs font-medium">{{ event.name }}</span>
                  <span class="shrink-0 font-mono text-[11px] text-muted-foreground">v{{ event.aggregate?.version }}</span>
                </span>
                <time class="mt-1 block font-mono text-[10px] text-muted-foreground" :title="formatDate(event.time)">{{ formatDayTime(event.time) }}</time>
              </span>
            </button>
          </li>
        </ol>

        <EmptyState v-else-if="!loading && !error" :icon="GitCommit" title="No events in this stream" />

        <Button
          v-if="nextCursor"
          variant="ghost"
          class="h-10 w-full rounded-none border-t text-xs text-muted-foreground"
          :disabled="loading"
          @click="load(true)"
        >
          <LoaderCircle v-if="loading" class="animate-spin" />
          {{ loading ? 'Loading…' : 'Load more events' }}
        </Button>
      </div>
    </DialogContent>
  </Dialog>
</template>
