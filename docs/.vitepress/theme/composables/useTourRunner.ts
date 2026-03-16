import { computed, ref } from 'vue'

export interface TourCheckResult {
  id: string
  label: string
  status: string
  message?: string
}

export interface TourRunResult {
  status: string
  stdout?: string
  stderr?: string
  durationMs: number
  compileErrors?: string[]
  checks?: TourCheckResult[]
}

export interface TourFileUpdate {
  path: string
  content: string
}

export function useTourRunner() {
  const pending = ref(false)
  const error = ref('')
  const result = ref<TourRunResult | null>(null)

  async function execute(lessonId: string, action: 'run' | 'check', files: TourFileUpdate[]) {
    pending.value = true
    error.value = ''

    try {
      const response = await fetch(`${runnerBaseUrl()}/api/execute`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          lessonId,
          action,
          files,
        }),
      })

      const payload = await response.json()
      if (!response.ok) {
        throw new Error(payload.error ?? 'Runner request failed')
      }

      result.value = payload as TourRunResult
      return result.value
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Runner request failed'
      result.value = null
      return null
    } finally {
      pending.value = false
    }
  }

  return {
    pending: computed(() => pending.value),
    error: computed(() => error.value),
    result: computed(() => result.value),
    execute,
  }
}

function runnerBaseUrl() {
  return import.meta.env.VITEPRESS_TOUR_RUNNER_URL || 'http://localhost:8090'
}
