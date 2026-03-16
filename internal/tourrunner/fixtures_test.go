package tourrunner

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLessonFixturesStarterFailsAndSolutionPasses(t *testing.T) {
	lessons, err := LoadLessons(testLessonsRoot(t))
	if err != nil {
		t.Fatalf("load lessons: %v", err)
	}

	repoRoot := testRepoRoot(t)
	for _, id := range []string{"quick-start", "first-aggregate", "events-and-state", "repository"} {
		lesson := lessons[id]
		t.Run(id, func(t *testing.T) {
			workspace := t.TempDir()
			if err := lesson.CopyWorkspace(workspace); err != nil {
				t.Fatalf("copy workspace: %v", err)
			}
			if err := rewriteLessonReplace(filepath.Join(workspace, "go.mod"), repoRoot); err != nil {
				t.Fatalf("rewrite replace: %v", err)
			}

			if err := runGoTest(workspace); err == nil {
				t.Fatalf("starter workspace unexpectedly passed")
			}

			if err := copyDir(filepath.Join(lesson.RootDir, "solution"), workspace); err != nil {
				t.Fatalf("copy solution: %v", err)
			}

			if err := runGoTest(workspace); err != nil {
				t.Fatalf("solved workspace failed: %v", err)
			}
		})
	}
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func rewriteLessonReplace(goModPath, repoRoot string) error {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return err
	}
	updated := strings.ReplaceAll(string(data), "/opt/goes", filepath.ToSlash(repoRoot))
	return os.WriteFile(goModPath, []byte(updated), 0o644)
}

func runGoTest(workspace string) error {
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOSUMDB=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &execError{err: err, output: string(output)}
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

type execError struct {
	err    error
	output string
}

func (e *execError) Error() string {
	return e.err.Error() + ": " + e.output
}
