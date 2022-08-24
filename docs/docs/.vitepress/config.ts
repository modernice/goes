import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'goes',
  description: 'Event-Sourcing Framework for Go.',

  // TODO(bounoable): Disable after completing the documentation.
  ignoreDeadLinks: true,

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
            { text: 'Introduction', link: '/guide/aggregates/' },
            {
              text: 'Creating an Aggregate',
              link: '/guide/aggregates/#creating-an-aggregate',
            },
            { text: 'Repositories', link: '/guide/aggregates/repositories' },
            {
              text: 'Event Handling',
              link: '/guide/aggregates/event-handling',
            },
            {
              text: 'Command Handling',
              link: '/guide/aggregates/command-handling',
            },
          ],
        },
        {
          text: 'Projections',
          collapsible: true,
          items: [
            { text: 'Introduction', link: '/guide/projections/' },
            { text: 'Scheduling', link: '/guide/projections/scheduling' },
            { text: 'Projection Jobs', link: '/guide/projections/jobs' },
            { text: 'Projection Service', link: '/guide/projections/service' },
            {
              text: 'Best Practices',
              link: '/guide/projections/best-practices',
            },
          ],
        },
        {
          text: 'Commands',
          collapsible: true,
          items: [
            { text: 'Introduction', link: '/guide/commands/' },
            {
              text: 'Defining Commands',
              link: '/guide/commands/#defining-commands',
            },
            {
              text: 'Command Registry',
              link: '/guide/command/command-registry',
            },
            { text: 'Command Bus', link: '/guide/commands/command-bus' },
            {
              text: 'Command Handling',
              link: '/guide/commands/command-handling',
            },
          ],
        },
        {
          text: 'Process Manages / SAGAs',
          collapsible: true,
          items: [{ text: 'TODO', link: '/' }],
        },
        {
          text: 'Tools & Helpers',
          collapsible: true,
          collapsed: true,
          items: [
            { text: 'Aggregate Lookup', link: '/guide/tools/aggregate-lookup' },
            {
              text: 'Streaming Helpers',
              link: '/guide/tools/streaming-helpers',
            },
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
            { text: 'Authorization', link: '/guide/modules/authorization' },
          ],
        },
        {
          text: 'Backends – Event Bus',
          collapsible: true,
          collapsed: true,
          items: [
            { text: 'In-Memory', link: '/guide/backends/event-bus/nats' },
            { text: 'NATS', link: '/guide/backends/event-bus/nats' },
          ],
        },
        {
          text: 'Backends – Event Store',
          collapsible: true,
          collapsed: true,
          items: [
            {
              text: 'In-Memory',
              link: '/guide/backends/event-store/nats',
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
  },
})
