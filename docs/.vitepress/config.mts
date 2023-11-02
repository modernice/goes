import { defineConfig } from 'vitepress'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  title: 'modernice/goes',
  lang: 'en-US',
  description: 'A distributed Event-Sourcing framework for Go',
  themeConfig: {
    nav: [
      { text: 'Home', link: '/' },
      {
        text: 'Docs',
        items: [
          {
            text: 'Getting Started',
            link: '/getting-started/',
          },
        ],
      },
      { text: 'Examples', link: '/markdown-examples' },
    ],

    sidebar: [
      {
        text: 'Introduction',
        collapsed: true,
        items: [
          { text: 'What is goes?', link: '/introduction/' },
          { text: 'Why Event-Sourcing?', link: '/introduction/event-sourcing' },
        ],
      },
      {
        text: 'Getting Started',
        items: [
          { text: 'Basic Concepts', link: '/getting-started/basic-concepts' },
          {
            text: 'Setting Up Your First Project',
            link: '/getting-started/first-project',
          },
        ],
      },
      {
        text: 'Core Components',
        items: [
          {
            text: 'Event System',
            collapsed: true,
            items: [
              {
                text: 'Overview',
                link: '/core/events/',
              },
              {
                text: 'Working with Events',
                link: '/core/events/working-with-events',
              },
              {
                text: 'Advanced Features',
                link: '/core/events/advanced-features',
              },
            ],
          },
          {
            text: 'Command System',
            collapsed: true,
            items: [
              {
                text: 'Overview',
                link: '/core/commands/',
              },
              {
                text: 'Defining Commands',
                link: '/core/commands/defining-commands',
              },
              {
                text: 'Command Handlers',
                link: '/core/commands/command-handlers',
              },
            ],
          },
          {
            text: 'Aggregate Framework',
            collapsed: true,
            items: [
              {
                text: 'Overview',
                link: '/core/aggregates/',
              },
              {
                text: 'Designing Aggregates',
                link: '/core/aggregates/designing-aggregates',
              },
              {
                text: 'Aggregate Lifecycle',
                link: '/core/aggregates/aggregate-lifecycle',
              },
            ],
          },
          {
            text: 'Projection Toolkit',
            collapsed: true,
            items: [
              {
                text: 'Overview',
                link: '/core/projections/',
              },
              {
                text: 'Creating Projections',
                link: '/core/projections/creating-projections',
              },
              {
                text: 'Projection Management',
                link: '/core/projections/projection-management',
              },
            ],
          },
        ],
      },
      {
        text: 'Helpers',
        items: [
          {
            text: 'Streams',
            link: '/helpers/streams',
            collapsed: true,
            items: [
              {
                text: 'All',
                link: '/helpers/streams#all',
              },
              {
                text: 'Drain',
                link: '/helpers/streams#drain',
              },
              {
                text: 'ForEach',
                link: '/helpers/streams#foreach',
              },
              {
                text: 'New',
                link: '/helpers/streams#new',
              },
              {
                text: 'Take',
                link: '/helpers/streams#take',
              },
              {
                text: 'Walk',
                link: '/helpers/streams#walk',
              },
            ]
          },
        ]
      },
      {
        text: 'Integrations',
        items: [
          {
            text: 'Event Bus',
            collapsed: true,
            items: [
              {
                text: 'NATS',
                link: '/integrations/event-bus/nats',
              },
            ],
          },
          {
            text: 'Event Store',
            collapsed: true,
            items: [
              {
                text: 'In-Memory',
                link: '/integrations/event-store/in-memory',
              },
              {
                text: 'MongoDB',
                link: '/integrations/event-store/mongodb',
              },
              {
                text: 'Postgres (Beta)',
                link: '/integrations/event-store/postgres',
              },
            ],
          },
        ],
      },
      {
        text: 'Advanced Topics',
        items: [
          // {
          //   text: 'Process Managers',
          //   link: '/advanced/process-managers/',
          // },
          {
            text: 'Performance Tuning',
            link: '/advanced/performance-tuning/',
          },
        ],
      },
      {
        text: 'Guides and Examples',
        items: [
          {
            text: 'Quickstart Examples',
            link: '/guides/quickstart-examples/',
          },
          {
            text: 'Real-World Scenarios',
            link: '/guides/real-world-scenarios/',
          },
          {
            text: 'Tips and Tricks',
            link: '/guides/tips-and-tricks/',
          },
        ],
      },
      {
        text: 'Testing',
        items: [
          {
            text: 'Writing Tests',
            link: '/testing/writing-tests/',
          },
          {
            text: 'Mocking Components',
            link: '/testing/mocking-components/',
          },
          {
            text: 'Integration Testing',
            link: '/testing/integration-testing/',
          },
        ],
      },
      {
        text: 'API Reference',
        items: [
          {
            text: 'Event API',
            link: '/api/events/',
          },
          {
            text: 'Command API',
            link: '/api/commands/',
          },
          {
            text: 'Aggregate API',
            link: '/api/aggregates/',
          },
        ],
      },
      {
        text: 'Contributing',
        items: [
          {
            text: 'How to Contribute',
            link: '/contributing/',
          },
        ],
      },
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/modernice/goes' },
    ],
  },

  vue: {
    script: {
      propsDestructure: true,
    }
  },
})
