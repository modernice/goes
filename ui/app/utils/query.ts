export function routeQueryStrings(value: unknown): string[] {
  const values = Array.isArray(value) ? value : [value]
  return [...new Set(values.flatMap(item => typeof item === 'string' ? [item.trim()] : []).filter(Boolean))]
}
