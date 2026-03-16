<script setup lang="ts">
import { getTourLessons } from './lessons'

const lessons = getTourLessons()
</script>

<template>
  <section class="tour-landing">
    <div class="tour-hero">
      <p class="tour-kicker">Interactive Tour</p>
      <h1>Learn goes by editing real code</h1>
      <p class="tour-summary">
        The Tour is the hands-on lane: short lessons, locked scaffolding, live
        output, and checks that verify each concept before you move on.
      </p>

      <div class="tour-meta-grid">
        <div class="tour-meta-card">
          <strong>4 runnable lessons</strong>
          <span>Start with a tiny todo aggregate and end with repository replay.</span>
        </div>
        <div class="tour-meta-card">
          <strong>1-2 editable files</strong>
          <span>Focus on the important code instead of project setup.</span>
        </div>
        <div class="tour-meta-card">
          <strong>Existing docs stay useful</strong>
          <span>Each lesson links back to the full tutorial and guide material.</span>
        </div>
      </div>
    </div>

    <div class="tour-steps">
      <a
        v-for="lesson in lessons"
        :key="lesson.id"
        :href="`/tour/${lesson.id}`"
        class="tour-step-card"
      >
        <span class="tour-step-index">{{ lesson.order }}</span>
        <div>
          <h2>{{ lesson.title }}</h2>
          <p>{{ lesson.summary }}</p>
        </div>
      </a>
    </div>
  </section>
</template>
