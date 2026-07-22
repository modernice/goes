<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <div
    :class="cn('grid size-6 shrink-0 place-items-center rounded-md bg-primary text-primary-foreground', props.class)"
    aria-hidden="true"
  >
    <svg viewBox="0 0 24 24" class="size-[72%]" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
      <path d="M9 7h8" />
      <path d="M5 12h14" />
      <path d="M7 17h6" />
    </svg>
  </div>
</template>
