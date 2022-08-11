<script lang="ts" setup>
import DefaultTheme from 'vitepress/theme'

const { Layout } = DefaultTheme
</script>

<template>
  <Layout>
    <slot />
  </Layout>
</template>

<style lang="postcss">
:root {
  --vp-c-brand: #c87833;
  --vp-c-brand-light: #d8925c;
  --vp-c-brand-lighter: #ffbb81;
  --vp-c-brand-dark: #b36826;
  --vp-c-brand-darker: #000;

  /* --vp-c-white-soft: #f9f9f9;
  --vp-c-white-mute: #f1f1f1;

  --vp-c-black: #131317;
  --vp-c-black-pure: #000000;
  --vp-c-black-soft: #18181b;
  --vp-c-black-mute: #27272a; */
}
</style>
