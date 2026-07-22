<script setup lang="ts">
import { Check, Copy } from '@lucide/vue'
import { Button } from '@/components/ui/button'

const props = withDefaults(defineProps<{
  text: string
  label?: string
  withText?: boolean
}>(), {
  label: 'Copy',
  withText: false,
})

const copied = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined

async function copy(): Promise<void> {
  await navigator.clipboard.writeText(props.text)
  copied.value = true
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => { copied.value = false }, 1500)
}

onBeforeUnmount(() => { if (timer) clearTimeout(timer) })
</script>

<template>
  <Button
    :variant="withText ? 'outline' : 'ghost'"
    :size="withText ? 'xs' : 'icon-xs'"
    :title="label"
    :aria-label="label"
    @click.stop="copy"
  >
    <Check v-if="copied" class="text-success" />
    <Copy v-else />
    <template v-if="withText">{{ copied ? 'Copied' : label }}</template>
  </Button>
</template>
