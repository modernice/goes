<script setup lang="ts">
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { Facet } from '~/types/api'

const allValue = '__goes_all__'

const props = defineProps<{
  id?: string
  modelValue: string[]
  facets: Facet[]
  allLabel: string
  selectedLabel: string
}>()

const emit = defineEmits<{ 'update:modelValue': [value: string[]] }>()

const triggerLabel = computed(() => {
  if (!props.modelValue.length) return props.allLabel
  if (props.modelValue.length === 1) return props.modelValue[0]
  return `${props.modelValue.length} ${props.selectedLabel} selected`
})

function updateSelection(value: unknown): void {
  const values = (Array.isArray(value) ? value : [value])
    .filter((item): item is string => typeof item === 'string')
  emit('update:modelValue', values.includes(allValue) ? [] : values)
}
</script>

<template>
  <Select :model-value="modelValue" multiple @update:model-value="updateSelection">
    <SelectTrigger :id="id" class="w-full" :title="modelValue.join(', ') || allLabel">
      <SelectValue class="min-w-0 *:truncate">{{ triggerLabel }}</SelectValue>
    </SelectTrigger>
    <SelectContent position="popper" align="start">
      <SelectItem :value="allValue">{{ allLabel }}</SelectItem>
      <SelectItem
        v-for="facet in facets"
        :key="facet.value"
        :value="facet.value"
        class="[&>span:last-child]:min-w-0 [&>span:last-child]:flex-1"
      >
        <span class="min-w-0 truncate font-mono text-xs">{{ facet.value }}</span>
        <span class="ml-auto shrink-0 pl-3 font-mono text-[11px] text-muted-foreground tabular-nums">{{ facet.count }}</span>
      </SelectItem>
    </SelectContent>
  </Select>
</template>
