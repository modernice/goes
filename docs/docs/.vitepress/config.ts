import { defineConfig, HeadConfig } from 'vitepress'

const domain = String(process.env.DOCS_DOMAIN || 'goes.modernice.dev')

const extensions = ['outbound-links']

if (domain.includes('localhost')) {
  extensions.push('local')
}

const plausibleScript = `https://plausible.io/js/plausible.${extensions.join(
  '.'
)}.js`

const head: HeadConfig[] = [
  [
    'script',
    {
      src: plausibleScript,
      defer: '',
      'data-domain': domain,
    },
  ],
]

export default defineConfig({
  title: 'goes',
  description: 'Event-Sourcing Framework for Go.',

  // TODO(bounoable): Disable after completing the documentation.
  ignoreDeadLinks: true,

  head,

  themeConfig: {
    siteTitle: 'goes',

    socialLinks: [
      { link: 'https://github.com/modernice/goes', icon: 'github' },
    ],

    nav: [{ text: 'CLI Reference', link: '/cli/' }],

    sidebar: {
      '/cli/': [{ text: 'CLI', items: [] }],

      '': [
        {
          text: 'Getting Started',
          collapsible: true,
          items: [
            { text: 'Introduction', link: '/guide/introduction' },
            { text: 'Installation', link: '/guide/installation' },
          ],
        },
        {
          text: 'Events',
          collapsible: true,
          items: [
            { text: 'Introduction', link: '/guide/events/' },
            { text: 'Creating Events', link: '/guide/events/creating-events' },
            {
              text: 'Publish & Subscribe',
              link: '/guide/events/publish-subscribe',
            },
            { text: 'Event Store', link: '/guide/events/event-store' },
            { text: 'Event Registry', link: '/guide/events/event-registry' },
          ],
        },
        {
          text: 'Aggregates',
          collapsible: true,
          items: [
            {
              text: 'README',
              link: '/guide/aggregates/readme.md',
            },
            // {
            //   text: 'Introduction',
            //   link: '/guide/aggregates/',
            // },
            // {
            //   text: 'Creating an Aggregate',
            //   link: '/guide/aggregates/create',
            // },
            // { text: 'Repositories', link: '/guide/aggregates/repositories' },
            // {
            //   text: 'Event Handling',
            //   link: '/guide/aggregates/event-handling',
            // },
            // {
            //   text: 'Command Handling',
            //   link: '/guide/aggregates/command-handling',
            // },
          ],
        },
        {
          text: 'Projections',
          collapsible: true,
          items: [
            { text: 'README', link: '/guide/projections/readme' },
            // { text: 'Introduction', link: '/guide/projections/' },
            // { text: 'Scheduling', link: '/guide/projections/scheduling' },
            // { text: 'Projection Jobs', link: '/guide/projections/jobs' },
            // { text: 'Projection Service', link: '/guide/projections/service' },
            // {
            //   text: 'Best Practices',
            //   link: '/guide/projections/best-practices',
            // },
          ],
        },
        {
          text: 'Commands',
          collapsible: true,
          items: [
            { text: 'README', link: '/guide/commands/readme' },
            // { text: 'Introduction', link: '/guide/commands/' },
            // {
            //   text: 'Defining Commands',
            //   link: '/guide/commands/#defining-commands',
            // },
            // {
            //   text: 'Command Registry',
            //   link: '/guide/command/command-registry',
            // },
            // { text: 'Command Bus', link: '/guide/commands/command-bus' },
            // {
            //   text: 'Command Handling',
            //   link: '/guide/commands/command-handling',
            // },
          ],
        },
        {
          text: 'Process Manages / SAGAs',
          collapsible: true,
          items: [{ text: 'README', link: '/guide/process-managers/readme' }],
        },
        {
          text: 'Tools & Helpers',
          collapsible: true,
          collapsed: true,
          items: [
            // { text: 'Aggregate Lookup', link: '/guide/tools/aggregate-lookup' },
            // {
            //   text: 'Streaming Helpers',
            //   link: '/guide/tools/streaming-helpers',
            // },
          ],
        },
        {
          text: 'Testing',
          collapsible: true,
          collapsed: true,
          items: [],
        },
        {
          text: 'Modules',
          collapsible: true,
          collapsed: true,
          items: [
            // { text: 'Authorization', link: '/guide/modules/authorization' },
          ],
        },
        {
          text: 'Backends',
          collapsible: true,
          collapsed: true,
          items: [
            {
              text: 'Event Bus',
              items: [
                {
                  text: 'In-Memory',
                  link: '/guide/backends/event-bus/in-memory',
                },
                { text: 'NATS', link: '/guide/backends/event-bus/nats' },
              ],
            },
            {
              text: 'Event Store',
              items: [
                {
                  text: 'In-Memory',
                  link: '/guide/backends/event-store/in-memory',
                },
                {
                  text: 'MongoDB',
                  link: '/guide/backends/event-store/mongodb',
                },
                {
                  text: 'Postgres',
                  link: '/guide/backends/event-store/postgres',
                },
              ],
            },
          ],
        },
      ],
    },
  },
})
