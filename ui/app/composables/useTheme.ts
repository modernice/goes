export type ThemeMode = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'goes-ui-theme'

function systemTheme(): 'light' | 'dark' {
  if (!import.meta.client) return 'dark'
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export function useTheme() {
  const mode = useState<ThemeMode>('theme:mode', () => {
    if (!import.meta.client) return 'system'
    const saved = localStorage.getItem(STORAGE_KEY)
    return saved === 'light' || saved === 'dark' ? saved : 'system'
  })

  const resolved = useState<'light' | 'dark'>('theme:resolved', () =>
    mode.value === 'system' ? systemTheme() : mode.value)

  function apply(): void {
    if (!import.meta.client) return
    resolved.value = mode.value === 'system' ? systemTheme() : mode.value
    document.documentElement.classList.toggle('dark', resolved.value === 'dark')
  }

  function set(next: ThemeMode): void {
    mode.value = next
    if (import.meta.client) {
      if (next === 'system') localStorage.removeItem(STORAGE_KEY)
      else localStorage.setItem(STORAGE_KEY, next)
    }
    apply()
  }

  if (import.meta.client) {
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const onChange = () => { if (mode.value === 'system') apply() }
    onMounted(() => {
      apply()
      media.addEventListener('change', onChange)
    })
    onBeforeUnmount(() => media.removeEventListener('change', onChange))
  }

  return { mode, resolved, set }
}
