<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import goesLogo from '../../assets/goes_logo.png'

interface Feature {
  icon: string
  title: string
  description: string
}

const features: Feature[] = [
  {
    icon: '🚀',
    title: 'Event-Sourced Aggregates',
    description:
      'Embed a base type, register typed event handlers, and let the framework handle versioning, persistence, and replay.',
  },
  {
    icon: '🔌',
    title: 'Distributed Events',
    description:
      'Publish and subscribe to events with NATS or use MongoDB and PostgreSQL as event stores. Swap backends without changing application code.',
  },
  {
    icon: '🧠',
    title: 'Type-Safe Commands',
    description:
      'Dispatch and handle commands with full generic type safety. Synchronous or asynchronous — your choice.',
  },
  {
    icon: '🛠️',
    title: 'Projection Toolkit',
    description:
      'Build read models with continuous or periodic schedules. Automatic progress tracking, debouncing, and startup catch-up.',
  },
  {
    icon: '📦',
    title: 'Codec Registry',
    description:
      'Map event names to Go types for automatic serialization. JSON by default, swap to MessagePack or Protobuf with one option.',
  },
  {
    icon: '📸',
    title: 'Snapshots',
    description:
      'Capture aggregate state at a point in time. Replay only recent events instead of the full history.',
  },
  {
    icon: '✅',
    title: 'Testable',
    description:
      'Fluent test assertions for aggregates, in-memory backends for integration tests, and conformance suites for custom implementations.',
  },
  {
    icon: '🦾',
    title: 'Modular Design',
    description:
      'Use only what you need — the event system, commands, projections — and adopt more components as your project grows.',
  },
  {
    icon: '🔋',
    title: 'Batteries Included',
    description:
      'MongoDB, PostgreSQL, and NATS backends ready to go. In-memory implementations for testing. Zero external dependencies for prototyping.',
  },
]

let cleanup: (() => void) | null = null

onMounted(() => {
  const grid = document.querySelector('.features-grid')
  if (!grid) return

  const handler = (e: MouseEvent) => {
    const cards = grid.querySelectorAll('.feature-card') as NodeListOf<HTMLElement>
    cards.forEach((card) => {
      const rect = card.getBoundingClientRect()
      card.style.setProperty('--mouse-x', `${e.clientX - rect.left}px`)
      card.style.setProperty('--mouse-y', `${e.clientY - rect.top}px`)
    })
  }

  grid.addEventListener('mousemove', handler)
  cleanup = () => grid.removeEventListener('mousemove', handler)
})

onUnmounted(() => {
  cleanup?.()
})
</script>

<template>
  <div class="home-page">
    <!-- Hero -->
    <section class="hero">
      <div class="hero-content">
        <div class="hero-art">
          <img :src="goesLogo" alt="goes gopher logo" class="hero-logo" />
        </div>
        <h1 class="hero-title">
          <span class="hero-title-gradient">goes</span>
        </h1>
        <p class="hero-tagline">Event-Sourcing Framework for Go</p>
        <p class="hero-description">
          Build distributed, event-sourced applications with type-safe
          aggregates, commands, and projections.
        </p>
        <div class="hero-actions">
          <a href="/tutorial/" class="action-button primary">Start Tutorial</a>
          <a href="/getting-started/quick-start" class="action-button secondary">
            Quick Start
          </a>
          <a
            href="https://github.com/modernice/goes"
            class="action-button secondary"
            target="_blank"
            rel="noopener noreferrer"
          >
            GitHub
          </a>
        </div>
      </div>
    </section>

    <!-- Features -->
    <section class="features">
      <div class="features-grid">
        <div
          v-for="feature in features"
          :key="feature.title"
          class="feature-card"
        >
          <div class="feature-icon">{{ feature.icon }}</div>
          <h3 class="feature-title">{{ feature.title }}</h3>
          <p class="feature-description">{{ feature.description }}</p>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
/* ---- Layout ---- */
.home-page {
  max-width: 1152px;
  margin: 0 auto;
  padding: 0 24px;
}

/* ---- Hero ---- */
.hero {
  padding: 76px 0 64px;
  text-align: center;
}

.hero-content {
  max-width: 720px;
  margin: 0 auto;
}

.hero-art {
  display: flex;
  justify-content: center;
  margin-bottom: 24px;
}

.hero-logo {
  width: min(280px, 72vw);
  height: auto;
  display: block;
}

.hero-title {
  font-family: 'JetBrains Mono', var(--vp-font-family-mono);
  font-size: 64px;
  font-weight: 700;
  line-height: 1.1;
  letter-spacing: -0.04em;
  margin-bottom: 16px;
}

.hero-title-gradient {
  background: linear-gradient(135deg, #22d3ee 0%, #06b6d4 40%, #e4e4e7 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.hero-tagline {
  font-family: 'JetBrains Mono', var(--vp-font-family-mono);
  font-size: 20px;
  font-weight: 500;
  color: var(--vp-c-text-1);
  margin-bottom: 12px;
  letter-spacing: -0.02em;
}

.hero-description {
  font-family: var(--vp-font-family-base);
  font-size: 16px;
  line-height: 1.7;
  color: var(--vp-c-text-2);
  max-width: 560px;
  margin: 0 auto 32px;
}

/* ---- CTA Buttons ---- */
.hero-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
  flex-wrap: wrap;
}

.action-button {
  display: inline-flex;
  align-items: center;
  font-family: 'JetBrains Mono', var(--vp-font-family-mono);
  font-size: 14px;
  font-weight: 600;
  padding: 10px 24px;
  border-radius: 8px;
  text-decoration: none;
  transition: all 0.2s ease;
}

.action-button.primary {
  background-color: #22d3ee;
  color: #0a0a0a;
}

.action-button.primary:hover {
  background-color: #06b6d4;
}

.action-button.primary:active {
  background-color: #0891b2;
}

.action-button.secondary {
  background-color: #1a1a1a;
  color: #e4e4e7;
  border: 1px solid #27272a;
}

.action-button.secondary:hover {
  background-color: #27272a;
  border-color: #3f3f46;
}

/* Light mode button overrides */
html:not(.dark) .action-button.primary {
  background-color: #0891b2;
  color: #ffffff;
}

html:not(.dark) .action-button.primary:hover {
  background-color: #0e7490;
}

html:not(.dark) .action-button.secondary {
  background-color: #f4f4f5;
  color: #18181b;
  border-color: #d4d4d8;
}

html:not(.dark) .action-button.secondary:hover {
  background-color: #e4e4e7;
  border-color: #a1a1aa;
}

/* ---- Feature Cards ---- */
.features {
  padding: 0 0 80px;
}

.features-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

@media (max-width: 959px) {
  .features-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 639px) {
  .features-grid {
    grid-template-columns: 1fr;
  }

  .hero-title {
    font-size: 48px;
  }
}

.feature-card {
  position: relative;
  padding: 24px;
  border-radius: 12px;
  background-color: #111111;
  border: 1px solid #27272a;
  transition: all 0.3s ease;
  overflow: hidden;
}

/* Cursor-tracking glow */
.feature-card::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: 12px;
  opacity: 0;
  transition: opacity 0.3s ease;
  background: radial-gradient(
    600px circle at var(--mouse-x, 50%) var(--mouse-y, 50%),
    rgba(34, 211, 238, 0.06),
    transparent 40%
  );
  pointer-events: none;
}

.feature-card:hover {
  border-color: rgba(34, 211, 238, 0.3);
}

.feature-card:hover::before {
  opacity: 1;
}

/* Light mode cards */
html:not(.dark) .feature-card {
  background-color: #ffffff;
  border-color: #e4e4e7;
}

html:not(.dark) .feature-card:hover {
  border-color: rgba(8, 145, 178, 0.4);
  box-shadow: 0 4px 24px rgba(8, 145, 178, 0.08);
}

html:not(.dark) .feature-card::before {
  background: radial-gradient(
    600px circle at var(--mouse-x, 50%) var(--mouse-y, 50%),
    rgba(8, 145, 178, 0.04),
    transparent 40%
  );
}

.feature-icon {
  font-size: 28px;
  margin-bottom: 12px;
  line-height: 1;
}

.feature-title {
  font-family: 'JetBrains Mono', var(--vp-font-family-mono);
  font-size: 15px;
  font-weight: 600;
  color: var(--vp-c-text-1);
  margin-bottom: 8px;
  letter-spacing: -0.01em;
}

.feature-description {
  font-family: var(--vp-font-family-base);
  font-size: 14px;
  line-height: 1.6;
  color: var(--vp-c-text-2);
  margin: 0;
}
</style>
