import DefaultTheme from 'vitepress/theme'
import HomePage from './HomePage.vue'
import './style.css'

import type { Theme } from 'vitepress'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('HomePage', HomePage)
  },
} satisfies Theme
