export function formatNumber(value?: number): string {
  return new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 }).format(value || 0)
}

export function formatExactNumber(value?: number): string {
  return new Intl.NumberFormat().format(value || 0)
}

export function formatDate(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric', month: 'short', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  }).format(date)
}

export function formatDayTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric', month: 'short', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  }).format(date)
}

export function relativeTime(value?: string): string {
  if (!value) return 'No events yet'
  const seconds = Math.round((new Date(value).getTime() - Date.now()) / 1000)
  const formatter = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })
  const ranges: Array<[Intl.RelativeTimeFormatUnit, number]> = [
    ['year', 31_536_000], ['month', 2_592_000], ['week', 604_800],
    ['day', 86_400], ['hour', 3_600], ['minute', 60], ['second', 1],
  ]
  for (const [unit, size] of ranges) {
    if (Math.abs(seconds) >= size || unit === 'second') return formatter.format(Math.round(seconds / size), unit)
  }
  return 'just now'
}

export function shortID(value?: string): string {
  if (!value) return '—'
  return `${value.slice(0, 8)}…${value.slice(-4)}`
}

export function prettyJSON(value: unknown): string {
  return JSON.stringify(value === undefined ? null : value, null, 2)
}

export function payloadPreview(value: unknown): string {
  if (value === null || value === undefined) return 'null'
  const compact = JSON.stringify(value)
  return compact.length > 100 ? `${compact.slice(0, 97)}…` : compact
}
