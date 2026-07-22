<script setup lang="ts">
import { ChevronDown, CircleAlert, LogOut, Monitor, Moon, RefreshCw, ShieldAlert, Sun, TriangleAlert } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { ThemeMode } from '~/composables/useTheme'

const route = useRoute()
const { session, authenticationEnabled, logout } = useAuth()
const { stores, selectedStoreID, currentStore, onlineCount, error, load, select, reset } = useStores()
const theme = useTheme()

const storeSelection = computed({
  get: () => selectedStoreID.value,
  set: value => select(String(value ?? '')),
})

const themeSelection = computed({
  get: () => theme.mode.value as string,
  set: value => theme.set(value as ThemeMode),
})

const navigation = [
  { to: '/', label: 'Overview' },
  { to: '/events', label: 'Events' },
  { to: '/streams', label: 'Streams' },
]

function isActive(to: string): boolean {
  if (to === '/') return route.path === '/'
  return route.path.startsWith(to)
}

function driverLabel(driver?: string): string {
  return driver === 'postgres' ? 'PostgreSQL' : driver === 'mongo' ? 'MongoDB' : ''
}

onMounted(() => { void load().catch(() => undefined) })

async function signOut(): Promise<void> {
  await logout()
  reset()
  await navigateTo('/login')
}
</script>

<template>
  <div class="flex min-h-dvh flex-col">
    <header class="sticky top-0 z-40 border-b bg-background/90 backdrop-blur-md">
      <div class="mx-auto flex h-[52px] w-full max-w-[1440px] items-center gap-2 px-4 sm:px-6">
        <NuxtLink to="/" class="flex shrink-0 items-center gap-2 rounded-md outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
          <BrandMark />
          <span class="font-mono text-[13px] font-bold tracking-tight">goes</span>
        </NuxtLink>

        <svg viewBox="0 0 8 24" class="h-5 w-2 shrink-0 text-border" aria-hidden="true"><path d="M7 1L1 23" stroke="currentColor" stroke-linecap="round" /></svg>

        <Select v-model="storeSelection">
          <SelectTrigger
            class="h-8 min-w-0 gap-2 border-transparent bg-transparent px-2 text-[13px] font-medium shadow-none hover:bg-accent dark:bg-transparent dark:hover:bg-accent"
            :title="currentStore ? `${driverLabel(currentStore.driver)} · ${currentStore.status}` : undefined"
          >
            <span
              v-if="currentStore"
              :class="['size-1.5 shrink-0 rounded-full', currentStore.status === 'online' ? 'bg-success' : 'bg-destructive']"
            />
            <SelectValue class="truncate">{{ currentStore?.name || 'Select a store' }}</SelectValue>
          </SelectTrigger>
          <SelectContent position="popper" align="start" class="min-w-52">
            <SelectItem v-for="store in stores" :key="store.id" :value="store.id" class="gap-2.5 [&>span:last-child]:min-w-0 [&>span:last-child]:flex-1">
              <span
                :class="['size-1.5 shrink-0 rounded-full', store.status === 'online' ? 'bg-success' : 'bg-destructive']"
              />
              <span class="min-w-0 flex-1 truncate">{{ store.name }}</span>
              <span class="ml-3 shrink-0 font-mono text-[11px] text-muted-foreground">
                {{ driverLabel(store.driver) }}{{ store.status === 'online' ? '' : ' · offline' }}
              </span>
            </SelectItem>
          </SelectContent>
        </Select>

        <div class="ml-auto flex shrink-0 items-center gap-1">
          <div
            v-if="!authenticationEnabled"
            class="flex h-7 items-center gap-1.5 rounded-lg border border-warning/30 bg-warning/8 px-2.5 font-mono text-[11px] font-medium text-warning"
            title="Authentication is disabled. Do not expose this service in production."
          >
            <ShieldAlert :size="13" /> Unsecured
          </div>

          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="icon-sm" aria-label="Change theme">
                <Moon v-if="theme.resolved.value === 'dark'" />
                <Sun v-else />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="min-w-32">
              <DropdownMenuRadioGroup v-model="themeSelection">
                <DropdownMenuRadioItem value="light"><Sun /> Light</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="dark"><Moon /> Dark</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="system"><Monitor /> System</DropdownMenuRadioItem>
              </DropdownMenuRadioGroup>
            </DropdownMenuContent>
          </DropdownMenu>

          <DropdownMenu v-if="authenticationEnabled">
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="sm" class="gap-1.5 px-1.5">
                <span class="grid size-5 place-items-center rounded-full bg-secondary text-[10px] font-semibold uppercase">
                  {{ session?.username?.slice(0, 1) || '?' }}
                </span>
                <span class="hidden max-w-32 truncate text-[13px] sm:inline">{{ session?.username }}</span>
                <ChevronDown class="size-3! text-muted-foreground" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="min-w-44">
              <DropdownMenuLabel>
                <span class="block truncate text-xs font-medium text-foreground">{{ session?.username }}</span>
                <span class="text-[11px] font-normal">Static account</span>
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem @select="signOut"><LogOut /> Sign out</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <nav class="mx-auto flex h-9 w-full max-w-[1440px] items-stretch gap-4 overflow-x-auto px-4 sm:px-6" aria-label="Main">
        <NuxtLink
          v-for="item in navigation"
          :key="item.to"
          :to="item.to"
          :class="[
            'relative flex items-center px-1 font-mono text-[13px] tracking-tight whitespace-nowrap transition-colors outline-none focus-visible:text-foreground',
            isActive(item.to) ? 'font-medium text-foreground' : 'text-muted-foreground hover:text-foreground',
          ]"
        >
          {{ item.label }}
          <span v-if="isActive(item.to)" class="absolute inset-x-0 -bottom-px h-0.5 rounded-full bg-brand" />
        </NuxtLink>
      </nav>
    </header>

    <div v-if="!authenticationEnabled" class="border-b border-warning/30 bg-warning/8">
      <div class="mx-auto flex w-full max-w-[1440px] items-start gap-2.5 px-4 py-2.5 text-warning sm:items-center sm:px-6">
        <TriangleAlert :size="15" class="mt-0.5 shrink-0 sm:mt-0" />
        <p class="text-xs leading-relaxed">
          <strong class="font-medium">Authentication is disabled.</strong>
          Anyone who can reach this UI can inspect your event data. Do not use this configuration in production.
        </p>
      </div>
    </div>

    <div v-if="currentStore?.status === 'unavailable' || error" class="mx-auto w-full max-w-[1440px] px-4 pt-4 sm:px-6">
      <div class="flex items-center gap-3 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3">
        <CircleAlert :size="15" class="shrink-0 text-destructive" />
        <div class="min-w-0 flex-1">
          <p class="text-[13px] font-medium text-destructive">
            {{ currentStore?.status === 'unavailable' ? `${currentStore.name} is unavailable` : 'Could not load stores' }}
          </p>
          <p class="mt-0.5 truncate font-mono text-[11px] text-destructive/80" :title="currentStore?.error || error">
            {{ currentStore?.error || error }}
          </p>
        </div>
        <Button variant="outline" size="xs" class="shrink-0" @click="load(true)">
          <RefreshCw /> Retry
        </Button>
      </div>
    </div>

    <main class="mx-auto w-full max-w-[1440px] flex-1 px-4 py-5 sm:px-6">
      <slot />
    </main>

    <footer class="mt-auto border-t">
      <div class="mx-auto flex h-10 w-full max-w-[1440px] items-center justify-between gap-4 px-4 font-mono text-[11px] text-muted-foreground sm:px-6">
        <span class="truncate">goes · event store console</span>
        <span v-if="stores.length" class="shrink-0 tabular-nums">{{ onlineCount }}/{{ stores.length }} stores online</span>
      </div>
    </footer>
  </div>
</template>
