export function useLiveRefresh(callback: () => void | Promise<void>, interval = 5000) {
  const enabled = ref(false)
  let timer: ReturnType<typeof setInterval> | undefined

  function stop(): void {
    if (timer) clearInterval(timer)
    timer = undefined
  }

  watch(enabled, active => {
    stop()
    if (active) timer = setInterval(() => void callback(), interval)
  })

  onBeforeUnmount(stop)

  return { enabled, stop }
}
