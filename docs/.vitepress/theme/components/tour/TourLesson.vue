<script setup lang="ts">
import { computed, ref } from 'vue'
import { getTourLesson, getTourLessons } from './lessons'
import { useTourRunner } from '../../composables/useTourRunner'

const props = defineProps<{
  lessonId: string
}>()

const lesson = getTourLesson(props.lessonId)
const selectedPath = ref(lesson?.files[0]?.path ?? '')
const fileState = ref<Record<string, string>>(lesson ? { ...lesson.initialFiles } : {})
const { execute, pending, error, result } = useTourRunner()
const orderedLessons = getTourLessons()

const selectedFile = computed(() => lesson?.files.find((file) => file.path === selectedPath.value))
const editableFiles = computed(() => lesson?.files.filter((file) => file.editable) ?? [])
const previousLesson = computed(() => lesson && getTourLessonByOffset(lesson.order, -1))
const nextLesson = computed(() => lesson && getTourLessonByOffset(lesson.order, 1))

function updateCurrentFile(value: string) {
  if (!selectedPath.value) {
    return
  }
  fileState.value[selectedPath.value] = value
}

function resetLesson() {
  if (!lesson) {
    return
  }
  fileState.value = { ...lesson.initialFiles }
}

async function run(action: 'run' | 'check') {
  if (!lesson) {
    return
  }

  await execute(
    lesson.id,
    action,
    editableFiles.value.map((file) => ({
      path: file.path,
      content: fileState.value[file.path] ?? '',
    }))
  )
}

function getTourLessonByOffset(order: number, offset: number) {
  const targetOrder = order + offset
  return orderedLessons.find((item) => item.order === targetOrder)
}

function onEditorInput(event: Event) {
  updateCurrentFile((event.target as HTMLTextAreaElement).value)
}
</script>

<template>
  <section v-if="lesson" class="tour-lesson">
    <header class="tour-lesson-header">
      <div>
        <p class="tour-kicker">Lesson {{ lesson.order }}</p>
        <h1>{{ lesson.title }}</h1>
        <p class="tour-summary">{{ lesson.goal }}</p>
      </div>

      <div class="tour-doc-links">
        <a
          v-for="link in lesson.docsLinks"
          :key="link.href"
          :href="link.href"
          class="tour-doc-link"
        >
          {{ link.label }}
        </a>
      </div>
    </header>

    <div class="tour-workbench">
      <aside class="tour-sidebar">
        <strong>Files</strong>
        <button
          v-for="file in lesson.files"
          :key="file.path"
          type="button"
          class="tour-file-tab"
          :class="{ active: file.path === selectedPath }"
          @click="selectedPath = file.path"
        >
          <span>{{ file.label }}</span>
          <small>{{ file.editable ? 'editable' : 'locked' }}</small>
        </button>

        <div class="tour-actions">
          <button type="button" class="tour-button primary" :disabled="pending" @click="run('run')">
            {{ pending ? 'Working...' : lesson.run.label }}
          </button>
          <button type="button" class="tour-button" :disabled="pending" @click="run('check')">
            {{ lesson.check.label }}
          </button>
          <button type="button" class="tour-button ghost" :disabled="pending" @click="resetLesson">
            Reset lesson
          </button>
        </div>
      </aside>

      <div class="tour-editor-panel">
        <label class="tour-editor-label" :for="selectedPath">{{ selectedFile?.label }}</label>
        <textarea
          v-if="selectedFile?.editable"
          :id="selectedPath"
          class="tour-editor"
          :value="fileState[selectedPath]"
          spellcheck="false"
          @input="onEditorInput"
        />
        <pre v-else class="tour-code-preview"><code>{{ fileState[selectedPath] }}</code></pre>
      </div>
    </div>

    <section class="tour-results">
      <div class="tour-results-header">
        <h2>Runner output</h2>
        <p>Use Run for visible behavior and Check for hidden validation.</p>
      </div>

      <p v-if="error" class="tour-error">{{ error }}</p>

      <div v-if="result" class="tour-result-grid">
        <div class="tour-result-card">
          <strong>Status</strong>
          <span :class="`tour-status ${result.status}`">{{ result.status }}</span>
        </div>
        <div class="tour-result-card">
          <strong>Duration</strong>
          <span>{{ result.durationMs }} ms</span>
        </div>
      </div>

      <ul v-if="result?.checks?.length" class="tour-check-list">
        <li v-for="check in result.checks" :key="check.id" class="tour-check-item">
          <strong>{{ check.label }}</strong>
          <span :class="`tour-status ${check.status}`">{{ check.status }}</span>
          <p v-if="check.message">{{ check.message }}</p>
        </li>
      </ul>

      <ul v-if="result?.compileErrors?.length" class="tour-compile-errors">
        <li v-for="compileError in result.compileErrors" :key="compileError">{{ compileError }}</li>
      </ul>

      <div class="tour-output-grid">
        <div>
          <strong>stdout</strong>
          <pre class="tour-console"><code>{{ result?.stdout || '(empty)' }}</code></pre>
        </div>
        <div>
          <strong>stderr</strong>
          <pre class="tour-console"><code>{{ result?.stderr || '(empty)' }}</code></pre>
        </div>
      </div>
    </section>

    <nav class="tour-pager">
      <a v-if="previousLesson" class="tour-pager-link" :href="`/tour/${previousLesson.id}`">
        &larr; {{ previousLesson.title }}
      </a>
      <a v-else class="tour-pager-link" href="/tour/">Back to Tour</a>

      <a v-if="nextLesson" class="tour-pager-link" :href="`/tour/${nextLesson.id}`">
        {{ nextLesson.title }} &rarr;
      </a>
      <a v-else class="tour-pager-link" href="/tutorial/">Continue to Tutorial</a>
    </nav>
  </section>

  <p v-else>Unknown lesson.</p>
</template>
