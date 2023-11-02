<script setup lang="ts">
import { computed } from 'vue'

const { title: config } = defineProps<{
  title: string | { as: string; text: string }
  description?: string
}>()

const title = computed(() =>
  typeof config === 'string' ? config : config.text
)

const as = computed(() => (typeof config === 'string' ? 'p' : config.as))
</script>

<template>
  <div :class="$style.card">
    <component :is="as" :class="$style.title">{{ title }}</component>
    <p v-if="description" v-text="description" :class="$style.description" />
  </div>
</template>

<style lang="postcss" module>
.card {
  @apply flex flex-col p-8 rounded-lg border border-transparent !transition-all !duration-200 hover:shadow-lg;
  background: var(--vp-c-bg-soft);

  &:hover {
    border-color: var(--vp-c-brand);
  }
}

.title {
  @apply !m-0 text-xl font-semibold;
}

.description {
  color: var(--vp-c-text-2);
}
</style>