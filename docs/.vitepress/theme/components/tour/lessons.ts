export interface TourDocLink {
  label: string
  href: string
}

export interface TourFile {
  path: string
  label: string
  editable: boolean
}

export interface TourAction {
  label: string
  command: string[]
}

export interface TourManifest {
  id: string
  title: string
  order: number
  goal: string
  summary: string
  docsLinks: TourDocLink[]
  files: TourFile[]
  run: TourAction
  check: TourAction
}

export interface TourLesson extends TourManifest {
  initialFiles: Record<string, string>
}

const manifests = import.meta.glob('../../../../../tour/lessons/*/lesson.json', {
  eager: true,
  import: 'default',
}) as Record<string, TourManifest>

const rawWorkspaceFiles = import.meta.glob('../../../../../tour/lessons/*/workspace/**/*', {
  eager: true,
  query: '?raw',
  import: 'default',
}) as Record<string, string>

const lessons = Object.entries(manifests)
  .map(([manifestPath, manifest]) => {
    const lessonId = lessonIdFromPath(manifestPath)
    const initialFiles = Object.fromEntries(
      manifest.files.map((file) => [
        file.path,
        rawWorkspaceFiles[workspaceKey(lessonId, file.path)] ?? '',
      ])
    )

    return {
      ...manifest,
      initialFiles,
    } satisfies TourLesson
  })
  .sort((a, b) => a.order - b.order)

export function getTourLessons(): TourLesson[] {
  return lessons
}

export function getTourLesson(id: string): TourLesson | undefined {
  return lessons.find((lesson) => lesson.id === id)
}

function lessonIdFromPath(path: string): string {
  return path.split('/').at(-2) ?? ''
}

function workspaceKey(lessonId: string, filePath: string): string {
  return `../../../../../tour/lessons/${lessonId}/workspace/${filePath}`
}
