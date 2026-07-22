<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{
  icon?: Component
  title: string
  hint?: string
}>()
</script>

<template>
  <div class="flex flex-col items-center justify-center px-6 py-16 text-center">
    <div v-if="icon" class="mb-3 grid size-9 place-items-center rounded-lg border border-dashed text-muted-foreground">
      <component :is="icon" :size="16" />
    </div>
    <p class="text-[13px] font-medium">{{ title }}</p>
    <p v-if="hint" class="mt-1 text-xs text-muted-foreground">{{ hint }}</p>
    <slot />
  </div>
</template>
