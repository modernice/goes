package tourrunner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLessons(t *testing.T) {
	lessons, err := LoadLessons(testLessonsRoot(t))
	if err != nil {
		t.Fatalf("load lessons: %v", err)
	}

	if got, want := len(lessons), 4; got != want {
		t.Fatalf("loaded %d lessons, want %d", got, want)
	}

	for _, id := range []string{"quick-start", "first-aggregate", "events-and-state", "repository"} {
		lesson, ok := lessons[id]
		if !ok {
			t.Fatalf("missing lesson %q", id)
		}
		if len(lesson.EditableFiles) == 0 || len(lesson.EditableFiles) > 2 {
			t.Fatalf("lesson %q has %d editable files", id, len(lesson.EditableFiles))
		}

		for path, file := range lesson.EditableFiles {
			solutionPath := filepath.Join(lesson.RootDir, "solution", filepath.FromSlash(path))
			if _, err := os.Stat(solutionPath); err != nil {
				t.Fatalf("lesson %q missing solution for %q: %v", id, file.Path, err)
			}
		}
	}
}

func testLessonsRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Join(wd, "..", "..", "tour", "lessons")
}
