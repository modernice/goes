import type { APIErrorBody } from '~/types/api'

export class EventStoreAPIError extends Error {
  readonly status: number
  readonly code?: string

  constructor(message: string, status: number, code?: string) {
    super(message)
    this.name = 'EventStoreAPIError'
    this.status = status
    this.code = code
  }
}

export function useEventStoreAPI() {
  async function request<T>(path: string, options: Parameters<typeof $fetch<T>>[1] = {}): Promise<T> {
    try {
      return await $fetch<T>(path, {
        credentials: 'same-origin',
        retry: 0,
        ...options,
      })
    } catch (cause) {
      const error = cause as {
        status?: number
        statusCode?: number
        data?: APIErrorBody
        message?: string
      }
      const status = error.statusCode || error.status || 500
      throw new EventStoreAPIError(
        error.data?.error?.message || error.message || 'The request failed.',
        status,
        error.data?.error?.code,
      )
    }
  }

  return { request }
}
