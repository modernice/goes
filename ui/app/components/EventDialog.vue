<script setup lang="ts">
import { ArrowUpRight, GitBranch, TriangleAlert } from '@lucide/vue'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import type { StoredEvent } from '~/types/api'
import { formatDate, prettyJSON, relativeTime } from '~/utils/format'

defineProps<{ event: StoredEvent | null }>()
const emit = defineEmits<{ close: [], openStream: [event: StoredEvent] }>()
</script>

<template>
  <Dialog :open="!!event" @update:open="open => { if (!open) emit('close') }">
    <DialogContent class="flex max-h-[calc(100dvh-1rem)] max-w-3xl flex-col gap-0 overflow-hidden p-0 sm:max-h-[min(85vh,760px)]">
      <header class="shrink-0 border-b px-5 py-4 pr-14">
        <p class="text-[11px] font-medium text-muted-foreground">Event</p>
        <DialogTitle class="mt-0.5 font-mono text-[15px] leading-snug font-semibold break-all">{{ event?.name }}</DialogTitle>
        <div class="mt-1 flex items-center gap-1">
          <DialogDescription class="truncate font-mono text-[11px] text-muted-foreground" :title="event?.id">{{ event?.id }}</DialogDescription>
          <CopyButton v-if="event" :text="event.id" label="Copy event ID" />
        </div>
      </header>

      <div v-if="event" class="min-h-0 flex-1 overflow-y-auto">
        <dl class="grid grid-cols-2 gap-x-4 gap-y-4 px-5 py-4">
          <div>
            <dt class="text-[11px] text-muted-foreground">Occurred</dt>
            <dd class="mt-1 text-xs">{{ formatDate(event.time) }}</dd>
            <dd class="mt-0.5 text-[11px] text-muted-foreground">{{ relativeTime(event.time) }}</dd>
          </div>
          <div v-if="event.aggregate">
            <dt class="text-[11px] text-muted-foreground">Aggregate</dt>
            <dd class="mt-1 flex items-baseline gap-1.5 text-xs">
              <span class="truncate">{{ event.aggregate.name }}</span>
              <span class="shrink-0 font-mono text-[11px] text-muted-foreground">v{{ event.aggregate.version }}</span>
            </dd>
            <dd class="mt-0.5">
              <button
                type="button"
                class="inline-flex items-center gap-0.5 text-[11px] text-brand transition-colors outline-none hover:underline focus-visible:underline"
                @click="emit('openStream', event)"
              >
                <GitBranch :size="11" /> View stream <ArrowUpRight :size="11" />
              </button>
            </dd>
          </div>
          <div v-if="event.aggregate" class="col-span-2">
            <dt class="text-[11px] text-muted-foreground">Aggregate ID</dt>
            <dd class="mt-1 flex items-center gap-1">
              <code class="font-mono text-[11px] break-all">{{ event.aggregate.id }}</code>
              <CopyButton :text="event.aggregate.id" label="Copy aggregate ID" />
            </dd>
          </div>
        </dl>

        <section class="border-t">
          <div class="flex h-11 items-center justify-between px-5">
            <h3 class="font-mono text-xs font-medium tracking-tight">Payload</h3>
            <CopyButton v-if="!event.dataDecodeError" :text="prettyJSON(event.data)" label="Copy JSON" with-text />
          </div>
          <div class="px-5 pb-5">
            <div v-if="event.dataDecodeError" class="flex gap-2.5 rounded-lg border border-warning/30 bg-warning/8 p-3.5">
              <TriangleAlert :size="15" class="mt-0.5 shrink-0 text-warning" />
              <div class="min-w-0">
                <p class="text-xs font-medium text-warning">Payload could not be decoded</p>
                <p class="mt-1 font-mono text-[11px] break-all text-warning/80">{{ event.dataDecodeError }}</p>
                <p class="mt-2 text-[11px] text-muted-foreground">Register this event type in the codec passed to <code class="font-mono">eventstoreui.WithEncoding</code> to decode it.</p>
              </div>
            </div>
            <div v-else class="rounded-lg border bg-muted/40 p-4 dark:bg-muted/20">
              <JsonView :value="event.data" />
            </div>
          </div>
        </section>
      </div>
    </DialogContent>
  </Dialog>
</template>
