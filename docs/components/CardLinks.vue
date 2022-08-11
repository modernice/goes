<template>
  <div :class="$style.links">
    <slot />
  </div>
</template>

<style lang="postcss" module>
.links {
  @apply grid gap-2 sm:grid-cols-2 py-4;
}
</style>
