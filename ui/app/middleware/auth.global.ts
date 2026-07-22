export default defineNuxtRouteMiddleware(async (to) => {
  if (import.meta.server) return

  const { authenticated, resolve } = useAuth()
  try {
    await resolve()
  } catch {
    if (to.path !== '/login') return navigateTo('/login')
    return
  }

  if (!authenticated.value && to.path !== '/login') return navigateTo('/login')
  if (authenticated.value && to.path === '/login') return navigateTo('/')
})
