<script setup lang="ts">
import { CircleAlert, LoaderCircle } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

definePageMeta({ layout: 'auth', title: 'Sign in' })
useHead({ title: 'Sign in · goes Event Store' })

const { login, pending } = useAuth()
const credentials = reactive({ username: '', password: '' })
const error = ref('')

async function submit(): Promise<void> {
  error.value = ''
  try {
    await login(credentials.username, credentials.password)
    credentials.password = ''
    await navigateTo('/')
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : 'Could not sign in.'
  }
}
</script>

<template>
  <main class="relative grid min-h-dvh place-items-center px-4 py-12">
    <div
      class="pointer-events-none absolute inset-0 bg-[radial-gradient(var(--border)_1px,transparent_1px)] [background-size:22px_22px] [mask-image:radial-gradient(ellipse_60%_60%_at_50%_40%,#000_10%,transparent_100%)]"
      aria-hidden="true"
    />

    <div class="relative w-full max-w-[340px]">
      <div class="mb-6 flex items-center justify-center gap-2.5">
        <BrandMark class="size-8 rounded-lg" />
        <div class="leading-tight">
          <p class="font-mono text-sm font-bold tracking-tight">goes</p>
          <p class="font-mono text-[10px] text-muted-foreground">event store console</p>
        </div>
      </div>

      <div class="rounded-xl bg-card p-5 ring-1 ring-foreground/10">
        <h1 class="font-mono text-[15px] font-bold tracking-tight">Sign in</h1>
        <p class="mt-1 text-[13px] text-muted-foreground">Browse events and aggregate streams across your configured stores.</p>

        <form class="mt-5 grid gap-3.5" @submit.prevent="submit">
          <div class="grid gap-1.5">
            <Label for="username" class="text-xs">Username</Label>
            <Input id="username" v-model="credentials.username" class="h-9" autocomplete="username" autofocus required />
          </div>
          <div class="grid gap-1.5">
            <Label for="password" class="text-xs">Password</Label>
            <Input id="password" v-model="credentials.password" class="h-9" type="password" autocomplete="current-password" required />
          </div>

          <div v-if="error" class="flex items-center gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2.5">
            <CircleAlert :size="14" class="shrink-0 text-destructive" />
            <p class="text-xs text-destructive">{{ error }}</p>
          </div>

          <Button class="mt-1 h-9 w-full" type="submit" :disabled="pending">
            <LoaderCircle v-if="pending" class="animate-spin" />
            {{ pending ? 'Signing in…' : 'Sign in' }}
          </Button>
        </form>
      </div>

      <p class="mt-5 text-center font-mono text-[11px] text-muted-foreground">
        signed http-only sessions
      </p>
    </div>
  </main>
</template>
