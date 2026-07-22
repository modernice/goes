<script setup lang="ts">
const props = defineProps<{ value: unknown }>()

function esc(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

// Full literal class names so Tailwind's scanner picks them up.
const TOKEN_CLASS = {
  key: 'text-syn-key',
  string: 'text-syn-string',
  number: 'text-syn-number',
  literal: 'text-syn-literal',
  punctuation: 'text-syn-punctuation',
} as const

function span(token: keyof typeof TOKEN_CLASS, text: string): string {
  return `<span class="${TOKEN_CLASS[token]}">${esc(text)}</span>`
}

const punct = (text: string): string => span('punctuation', text)

function render(value: unknown, indent: number): string {
  if (value === null || value === undefined) return span('literal', 'null')
  if (typeof value === 'boolean') return span('literal', String(value))
  if (typeof value === 'number') return span('number', String(value))
  if (typeof value === 'string') return span('string', JSON.stringify(value))

  const pad = '  '.repeat(indent)
  const childPad = '  '.repeat(indent + 1)

  if (Array.isArray(value)) {
    if (!value.length) return punct('[]')
    const items = value.map(item => childPad + render(item, indent + 1))
    return `${punct('[')}\n${items.join(`${punct(',')}\n`)}\n${pad}${punct(']')}`
  }

  if (typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>)
    if (!entries.length) return punct('{}')
    const items = entries.map(([key, item]) =>
      childPad + span('key', JSON.stringify(key)) + punct(': ') + render(item, indent + 1))
    return `${punct('{')}\n${items.join(`${punct(',')}\n`)}\n${pad}${punct('}')}`
  }

  return span('literal', String(value))
}

const html = computed(() => render(props.value, 0))
</script>

<template>
  <!-- eslint-disable-next-line vue/no-v-html — html is built from escaped tokens only -->
  <pre class="overflow-x-auto font-mono text-xs leading-[1.75] whitespace-pre"><code v-html="html" /></pre>
</template>
