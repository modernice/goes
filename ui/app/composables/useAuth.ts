import type { Session } from '~/types/api'

export function useAuth() {
  const session = useState<Session | null>('auth:session', () => null)
  const resolved = useState('auth:resolved', () => false)
  const pending = useState('auth:pending', () => false)
  const { request } = useEventStoreAPI()

  const authenticated = computed(() => session.value?.authenticated === true)
  const authenticationEnabled = computed(() => session.value?.authenticationEnabled !== false)

  async function resolve(force = false): Promise<Session> {
    if (resolved.value && !force && session.value) return session.value
    pending.value = true
    try {
      session.value = await request<Session>('/api/v1/auth/session')
      resolved.value = true
      return session.value
    } finally {
      pending.value = false
    }
  }

  async function login(username: string, password: string): Promise<void> {
    pending.value = true
    try {
      session.value = await request<Session>('/api/v1/auth/login', {
        method: 'POST',
        body: { username, password },
      })
      resolved.value = true
    } finally {
      pending.value = false
    }
  }

  async function logout(): Promise<void> {
    try {
      await request<void>('/api/v1/auth/logout', { method: 'POST' })
    } finally {
      session.value = { authenticated: false, authenticationEnabled: true }
      resolved.value = true
    }
  }

  return { session, authenticated, authenticationEnabled, pending, resolve, login, logout }
}
