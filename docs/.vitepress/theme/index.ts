import { h } from 'vue'
import DefaultTheme from 'vitepress/theme'
import HomePage from './HomePage.vue'
import AgencyCard from './components/AgencyCard.vue'
import './style.css'
import { inject } from '@vercel/analytics'

import type { Theme } from 'vitepress'

export default {
  extends: DefaultTheme,
  Layout: () =>
    h(DefaultTheme.Layout, null, {
      'aside-ads-before': () => h(AgencyCard),
    }),
  enhanceApp({ app }) {
    app.component('HomePage', HomePage)

    // Inject Vercel Analytics
    if (typeof window !== 'undefined') {
      inject()
    }
  },
} satisfies Theme
