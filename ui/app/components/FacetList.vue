<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'
import { ListFilter } from '@lucide/vue'
import { Skeleton } from '@/components/ui/skeleton'
import type { Facet } from '~/types/api'
import { formatNumber } from '~/utils/format'

const props = withDefaults(defineProps<{
  title: string
  items: Facet[]
  loading?: boolean
  limit?: number
  linkTo: (facet: Facet) => RouteLocationRaw
  emptyHint?: string
}>(), {
  limit: 7,
  emptyHint: 'Nothing recorded yet.',
})

const visible = computed(() => props.items.slice(0, props.limit))
const maxCount = computed(() => Math.max(...props.items.map(facet => facet.count), 1))

function widthOf(facet: Facet): string {
  return `${Math.max((facet.count / maxCount.value) * 100, 3)}%`
}
</script>

<template>
  <div class="flex flex-col overflow-hidden rounded-xl bg-card ring-1 ring-foreground/10">
    <div class="flex h-10 shrink-0 items-center justify-between border-b px-4">
      <h2 class="font-mono text-[13px] font-medium tracking-tight">{{ title }}</h2>
      <span v-if="items.length" class="font-mono text-[11px] text-muted-foreground tabular-nums">{{ items.length }}</span>
    </div>

    <div v-if="loading && !items.length" class="space-y-1.5 p-3">
      <Skeleton v-for="index in 5" :key="index" class="h-7" />
    </div>

    <ul v-else-if="visible.length" class="p-1.5">
      <li v-for="facet in visible" :key="facet.value">
        <NuxtLink
          :to="linkTo(facet)"
          class="group relative flex h-8 items-center gap-3 overflow-hidden rounded-md px-2.5 transition-colors hover:bg-accent"
        >
          <span
            class="absolute inset-y-1 left-0 rounded-sm bg-brand/8 transition-colors group-hover:bg-brand/14 dark:bg-brand/10 dark:group-hover:bg-brand/16"
            :style="{ width: widthOf(facet) }"
          />
          <span class="relative min-w-0 truncate font-mono text-xs" :title="facet.value">{{ facet.value }}</span>
          <span class="relative ml-auto shrink-0 font-mono text-[11px] text-muted-foreground tabular-nums" :title="`${facet.count}`">{{ formatNumber(facet.count) }}</span>
        </NuxtLink>
      </li>
    </ul>

    <EmptyState v-else :icon="ListFilter" :title="`No ${title.toLowerCase()}`" :hint="emptyHint" />
  </div>
</template>
