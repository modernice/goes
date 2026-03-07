package tourrunner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type Action string

const (
	ActionRun   Action = "run"
	ActionCheck Action = "check"
)

type FileUpdate struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type ExecuteRequest struct {
	LessonID string       `json:"lessonId"`
	Action   Action       `json:"action"`
	Files    []FileUpdate `json:"files"`
}

type Service struct {
	Lessons map[string]Lesson
	Exec    Executor
	TempDir string
	Timeout time.Duration
}

func (s Service) Execute(ctx context.Context, req ExecuteRequest) (RunResult, error) {
	if s.Exec == nil {
		return RunResult{}, fmt.Errorf("executor is not configured")
	}
	lesson, ok := s.Lessons[req.LessonID]
	if !ok {
		return RunResult{}, fmt.Errorf("unknown lesson %q", req.LessonID)
	}
	if req.Action != ActionRun && req.Action != ActionCheck {
		return RunResult{}, fmt.Errorf("unsupported action %q", req.Action)
	}

	workspace, cleanup, err := s.prepareWorkspace(lesson, req.Files)
	if err != nil {
		return RunResult{}, err
	}
	defer cleanup()

	command := lesson.Manifest.Run.Command
	if req.Action == ActionCheck {
		command = lesson.Manifest.Check.Command
	}

	execCtx, cancel := context.WithTimeout(ctx, timeoutOrDefault(s.Timeout, 60*time.Second))
	defer cancel()

	output, err := s.Exec.Run(execCtx, ExecSpec{
		WorkspaceDir: workspace,
		Command:      command,
		Timeout:      timeoutOrDefault(s.Timeout, 60*time.Second),
	})
	if err != nil {
		if execCtx.Err() != nil {
			return RunResult{}, execCtx.Err()
		}
		if IsTimeout(err) {
			return RunResult{}, err
		}
	}
	if req.Action == ActionCheck {
		return NewCheckResult(output.Stdout, output.Stderr, output.Duration, err), nil
	}
	return NewRunResult(output.Stdout, output.Stderr, output.Duration, err), nil
}

func (s Service) prepareWorkspace(lesson Lesson, files []FileUpdate) (string, func(), error) {
	base := s.TempDir
	if base == "" {
		base = os.TempDir()
	}

	workspace, err := os.MkdirTemp(base, "goes-tour-")
	if err != nil {
		return "", nil, fmt.Errorf("create temp workspace: %w", err)
	}

	cleanup := func() { _ = os.RemoveAll(workspace) }
	if err := lesson.CopyWorkspace(workspace); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("copy workspace: %w", err)
	}

	for _, file := range files {
		clean := cleanLessonPath(file.Path)
		if clean == "" {
			cleanup()
			return "", nil, fmt.Errorf("invalid file path %q", file.Path)
		}
		if _, ok := lesson.EditableFiles[clean]; !ok {
			cleanup()
			return "", nil, fmt.Errorf("file %q is not editable for %s", clean, lesson.Manifest.ID)
		}

		target := filepath.Join(workspace, filepath.FromSlash(clean))
		if err := os.WriteFile(target, []byte(file.Content), 0o644); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("write %q: %w", clean, err)
		}
	}

	if err := os.MkdirAll(filepath.Join(workspace, ".tmp"), 0o755); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("create workspace tmp dir: %w", err)
	}

	return workspace, cleanup, nil
}

func timeoutOrDefault(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func (s Service) LessonSummaries() []LessonManifest {
	items := make([]LessonManifest, 0, len(s.Lessons))
	for _, lesson := range s.Lessons {
		items = append(items, lesson.Manifest)
	}
	slices.SortFunc(items, func(a, b LessonManifest) int {
		if a.Order == b.Order {
			return strings.Compare(a.ID, b.ID)
		}
		return a.Order - b.Order
	})
	return items
}

func IsTimeout(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}
