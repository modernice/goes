<script lang="ts" setup>
import { computed } from 'vue'

const { title: config, link = '#' } = defineProps<{
  title: string | { as: string; text: string }
  link?: string
  description?: string
}>()

const title = computed(() =>
  typeof config === 'string' ? config : config.text
)

const as = computed(() => (typeof config === 'string' ? 'p' : config.as))

const isExternal = computed(() => link.startsWith('http'))
const target = computed(() => (isExternal.value ? '_blank' : ''))
const rel = computed(() => (isExternal.value ? 'noreferrer' : ''))
</script>

<template>
  <a :href="link" :class="$style.link" :target="target" :rel="rel">
    <component :is="as" :class="$style.title">{{ title }}</component>
    <p v-if="description" v-text="description" :class="$style.description" />
  </a>
</template>

<style lang="postcss" module>
.link {
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
