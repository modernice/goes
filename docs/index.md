---
layout: home
hero:
  name: goes
  text: Event-Sourcing Framework for Go
  tagline: Harness the power of event-driven architecture to build robust, scalable Go applications with ease.
  image: /banner-goes-framework.png  # Replace with the path to an actual image banner for visual appeal
  actions:
    - theme: brand
      text: Get Started
      link: /getting-started/first-project
      aria-label: Start learning about the goes framework
    - theme: secondary
      text: Why event-sourcing?
      link: /introduction/event-sourcing
      aria-label: Understand the benefits of event sourcing
    - theme: alt
      text: Examples
      link: /examples/
      aria-label: Browse goes framework examples
    - theme: alt
      text: GitHub
      link: https://github.com/modernice/goes
      aria-label: View the goes framework on GitHub
features:
  - icon: 🔌
    title: Event System
    details: Expand your system's capabilities with an event system that's both distributed and easy to integrate.
  - icon: 🧠
    title: Command System
    details: Streamline commands across services with a system that's simple to use and understand.
  - icon: 🚀
    title: Aggregate Framework
    details: Implement event sourcing in your domain with straightforward tools to manage state changes.
  - icon: 🛠️
    title: Projection Toolkit
    details: Maintain data consistency with projections that you can set up and run across various systems.
  - icon: 📊
    title: Process Managers (Planned)
    details: Simplify the coordination of complex transactions and services with a structured approach.
  - icon: 🔋
    title: Batteries Included
    details: Kick off your project with ready-to-go integrations for widely-used databases and message brokers.
  - icon: ✅
    title: Testable (Planned)
    details: Access a broad range of testing tools to help ensure your system works as intended.
  - icon: ️🦾
    title: Modular Design
    details: Pick and choose the components you need, and incorporate more as your project evolves.
  - icon: ⚡️
    title: Fast & Efficient
    details: Enjoy a performance-oriented design that processes data in streams for reduced memory usage.
---

<style lang="postcss">
:root {
	--brand-1: theme('colors.sky.400');
	--brand-2: theme('colors.sky.500');
	--brand-3: theme('colors.sky.600');

	--secondary-1: theme('colors.orange.600');
	--secondary-2: theme('colors.orange.700');
	--secondary-3: theme('colors.orange.800');

	--vp-c-brand-1: var(--brand-1);
	--vp-c-brand-2: var(--brand-2);
	--vp-c-brand-3: var(--brand-3);

  --vp-home-hero-name-color: transparent;
  --vp-home-hero-name-background: -webkit-linear-gradient(120deg, theme('colors.sky.500'), theme('colors.sky.50'));
}

.VPButton.secondary {
	/* @apply border-orange-700/75 hover:bg-orange-900 hover:border-orange-700; */
	@apply border;

	border-color: var(--secondary-1);

	&:hover {
		@apply text-white;
		border-color: var(--secondary-2);
		background-color: var(--secondary-2);
	}
}
</style>

