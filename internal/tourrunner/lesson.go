package tourrunner

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type LessonDocLink struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

type LessonFile struct {
	Path     string `json:"path"`
	Label    string `json:"label"`
	Editable bool   `json:"editable"`
}

type LessonAction struct {
	Label   string   `json:"label"`
	Command []string `json:"command"`
}

type LessonManifest struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Order     int             `json:"order"`
	Goal      string          `json:"goal"`
	Summary   string          `json:"summary"`
	DocsLinks []LessonDocLink `json:"docsLinks"`
	Files     []LessonFile    `json:"files"`
	Run       LessonAction    `json:"run"`
	Check     LessonAction    `json:"check"`
}

type Lesson struct {
	Manifest      LessonManifest
	RootDir       string
	WorkspaceDir  string
	EditableFiles map[string]LessonFile
}

func LoadLessons(root string) (map[string]Lesson, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read lessons dir: %w", err)
	}

	lessons := make(map[string]Lesson, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		lesson, err := loadLesson(filepath.Join(root, entry.Name()))
		if err != nil {
			return nil, err
		}
		lessons[lesson.Manifest.ID] = lesson
	}

	return lessons, nil
}

func loadLesson(root string) (Lesson, error) {
	manifestPath := filepath.Join(root, "lesson.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return Lesson{}, fmt.Errorf("read manifest %q: %w", manifestPath, err)
	}

	var manifest LessonManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Lesson{}, fmt.Errorf("parse manifest %q: %w", manifestPath, err)
	}

	lesson := Lesson{
		Manifest:      manifest,
		RootDir:       root,
		WorkspaceDir:  filepath.Join(root, "workspace"),
		EditableFiles: make(map[string]LessonFile),
	}

	if err := lesson.Validate(); err != nil {
		return Lesson{}, err
	}

	for _, file := range manifest.Files {
		if file.Editable {
			lesson.EditableFiles[file.Path] = file
		}
	}

	return lesson, nil
}

func (l Lesson) Validate() error {
	if l.Manifest.ID == "" {
		return fmt.Errorf("lesson %q: missing id", l.RootDir)
	}
	if l.Manifest.Title == "" {
		return fmt.Errorf("lesson %q: missing title", l.Manifest.ID)
	}
	if len(l.Manifest.Files) == 0 {
		return fmt.Errorf("lesson %q: no files configured", l.Manifest.ID)
	}
	if len(l.Manifest.Run.Command) == 0 {
		return fmt.Errorf("lesson %q: missing run command", l.Manifest.ID)
	}
	if len(l.Manifest.Check.Command) == 0 {
		return fmt.Errorf("lesson %q: missing check command", l.Manifest.ID)
	}

	editable := 0
	seen := make(map[string]struct{}, len(l.Manifest.Files))
	for _, file := range l.Manifest.Files {
		clean := cleanLessonPath(file.Path)
		if clean == "" {
			return fmt.Errorf("lesson %q: invalid file path %q", l.Manifest.ID, file.Path)
		}
		if _, ok := seen[clean]; ok {
			return fmt.Errorf("lesson %q: duplicate file path %q", l.Manifest.ID, file.Path)
		}
		seen[clean] = struct{}{}

		fullPath := filepath.Join(l.WorkspaceDir, filepath.FromSlash(clean))
		if _, err := os.Stat(fullPath); err != nil {
			return fmt.Errorf("lesson %q: missing workspace file %q: %w", l.Manifest.ID, clean, err)
		}

		if file.Editable {
			editable++
		}
	}

	if editable == 0 {
		return fmt.Errorf("lesson %q: at least one editable file required", l.Manifest.ID)
	}
	if editable > 2 {
		return fmt.Errorf("lesson %q: too many editable files for v1", l.Manifest.ID)
	}

	for _, action := range []LessonAction{l.Manifest.Run, l.Manifest.Check} {
		for _, arg := range action.Command {
			if strings.TrimSpace(arg) == "" {
				return fmt.Errorf("lesson %q: empty command argument", l.Manifest.ID)
			}
		}
	}

	return nil
}

func (l Lesson) ListedFiles() []LessonFile {
	files := slices.Clone(l.Manifest.Files)
	slices.SortFunc(files, func(a, b LessonFile) int {
		return strings.Compare(a.Path, b.Path)
	})
	return files
}

func (l Lesson) CopyWorkspace(dst string) error {
	return filepath.WalkDir(l.WorkspaceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(l.WorkspaceDir, path)
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

func cleanLessonPath(path string) string {
	if path == "" {
		return ""
	}

	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return ""
	}

	return clean
}
