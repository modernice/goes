import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'goes',
  description: 'Event-Sourcing Framework for Go',

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/logo.svg' }],
  ],

  themeConfig: {
    siteTitle: 'goes',

    nav: [
      { text: 'Getting Started', link: '/getting-started/introduction' },
      { text: 'Tutorial', link: '/tutorial/' },
      { text: 'Guide', link: '/guide/aggregates' },
      { text: 'Backends', link: '/backends/' },
    ],

    sidebar: [
      {
        text: 'Getting Started',
        items: [
          { text: 'Introduction', link: '/getting-started/introduction' },
          { text: 'Installation', link: '/getting-started/installation' },
          { text: 'Quick Start', link: '/getting-started/quick-start' },
        ],
      },
      {
        text: 'Tutorial',
        collapsed: true,
        items: [
          { text: 'Overview', link: '/tutorial/' },
          { text: '1. Project Setup', link: '/tutorial/01-project-setup' },
          { text: '2. Your First Aggregate', link: '/tutorial/02-first-aggregate' },
          { text: '3. Events & State', link: '/tutorial/03-events-and-state' },
          { text: '4. Codec Registry', link: '/tutorial/04-codec-registry' },
          { text: '5. Repositories', link: '/tutorial/05-repository' },
          { text: '6. Commands', link: '/tutorial/06-commands' },
          { text: '7. The Order', link: '/tutorial/07-order-aggregate' },
          { text: '8. The Customer', link: '/tutorial/08-customer-aggregate' },
          { text: '9. Projections', link: '/tutorial/09-projections' },
          { text: '10. Production Backends', link: '/tutorial/10-backends' },
          { text: '11. Testing', link: '/tutorial/11-testing' },
        ],
      },
      {
        text: 'Guide',
        items: [
          { text: 'Aggregates', link: '/guide/aggregates' },
          { text: 'Events', link: '/guide/events' },
          { text: 'Commands', link: '/guide/commands' },
          { text: 'Projections', link: '/guide/projections' },
          { text: 'Codec Registry', link: '/guide/codec' },
          { text: 'Snapshots', link: '/guide/snapshots' },
          { text: 'Testing', link: '/guide/testing' },
        ],
      },
      {
        text: 'Backends',
        items: [
          { text: 'Overview', link: '/backends/' },
          { text: 'MongoDB', link: '/backends/mongodb' },
          { text: 'PostgreSQL', link: '/backends/postgres' },
          { text: 'NATS', link: '/backends/nats' },
          { text: 'In-Memory', link: '/backends/in-memory' },
        ],
      },
      {
        text: 'Reference',
        items: [
          { text: 'Architecture', link: '/reference/architecture' },
          { text: 'Best Practices', link: '/reference/best-practices' },
        ],
      },
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/modernice/goes' },
    ],

    search: {
      provider: 'local',
    },

    editLink: {
      pattern: 'https://github.com/modernice/goes/edit/main/docs/:path',
    },
  },
})
