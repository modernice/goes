import DefaultTheme from 'vitepress/theme'
import HomePage from './HomePage.vue'
import './style.css'
import { inject } from '@vercel/analytics'

import type { Theme } from 'vitepress'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('HomePage', HomePage)
    
    // Inject Vercel Analytics
    if (typeof window !== 'undefined') {
      inject()
    }
  },
} satisfies Theme
