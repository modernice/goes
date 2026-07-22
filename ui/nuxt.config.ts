import tailwindcss from '@tailwindcss/vite'

const themeInit = `(function(){try{var m=localStorage.getItem('goes-ui-theme');var d=m==='dark'||(m!=='light'&&window.matchMedia('(prefers-color-scheme: dark)').matches);document.documentElement.classList.toggle('dark',d)}catch(e){}})()`

export default defineNuxtConfig({
  ssr: false,
  devtools: { enabled: true },
  compatibilityDate: '2026-07-01',
  modules: ['shadcn-nuxt'],
  shadcn: {
    prefix: '',
    componentDir: '@/components/ui',
  },
  css: ['~/assets/css/tailwind.css'],
  app: {
    head: {
      title: 'goes · Event Store',
      meta: [
        { name: 'description', content: 'A web console for goes event stores.' },
        { name: 'theme-color', media: '(prefers-color-scheme: light)', content: '#fafafa' },
        { name: 'theme-color', media: '(prefers-color-scheme: dark)', content: '#0a0a0a' },
      ],
      script: [{ innerHTML: themeInit }],
    },
  },
  vite: {
    plugins: [tailwindcss()],
    server: {
      proxy: {
        '/api': {
          target: process.env.GOES_UI_API_PROXY || 'http://127.0.0.1:8080',
          changeOrigin: true,
        },
      },
    },
  },
})
