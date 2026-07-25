<template>
  <a
    class="agency-card"
    href="https://modernice.design/en?utm_source=goes-docs&utm_content=sidebar"
    target="_blank"
    rel="noopener noreferrer"
  >
    <span class="agency-card-kicker">Built by</span>
    <span class="agency-card-name">
      modernice<span class="agency-card-domain">.design</span>
    </span>
    <span class="agency-card-text">
      The digital agency behind goes. We build web apps, AI solutions &amp;
      e-commerce platforms.
    </span>
    <span class="agency-card-cta">Work with us →</span>
  </a>
</template>

<style scoped>
.agency-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 16px;
  border-radius: 12px;
  background-color: #111111;
  border: 1px solid #27272a;
  text-decoration: none;
  transition: border-color 0.2s ease;
}

.agency-card:hover {
  border-color: rgba(34, 211, 238, 0.4);
}

.agency-card-kicker {
  font-family: var(--vp-font-family-mono);
  font-size: 10px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--vp-c-text-3);
}

.agency-card-name {
  font-family: var(--vp-font-family-mono);
  font-size: 15px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--vp-c-text-1);
}

.agency-card-domain {
  color: var(--vp-c-brand-1);
}

.agency-card-text {
  font-size: 12px;
  line-height: 1.5;
  color: var(--vp-c-text-2);
}

.agency-card-cta {
  font-family: var(--vp-font-family-mono);
  font-size: 12px;
  font-weight: 600;
  color: var(--vp-c-brand-1);
  transition: color 0.2s ease;
}

.agency-card:hover .agency-card-cta {
  color: var(--vp-c-brand-2);
}

/* Light mode */
html:not(.dark) .agency-card {
  background-color: #ffffff;
  border-color: #e4e4e7;
}

html:not(.dark) .agency-card:hover {
  border-color: rgba(8, 145, 178, 0.4);
  box-shadow: 0 4px 24px rgba(8, 145, 178, 0.08);
}
</style>
