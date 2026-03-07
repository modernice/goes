package tourrunner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type stubExecutor struct {
	run func(ctx context.Context, spec ExecSpec) (ExecOutput, error)
}

func (s stubExecutor) Run(ctx context.Context, spec ExecSpec) (ExecOutput, error) {
	return s.run(ctx, spec)
}

func TestServiceExecuteWritesEditedFiles(t *testing.T) {
	lessons, err := LoadLessons(testLessonsRoot(t))
	if err != nil {
		t.Fatalf("load lessons: %v", err)
	}

	var gotCommand []string
	service := Service{
		Lessons: lessons,
		Exec: stubExecutor{run: func(_ context.Context, spec ExecSpec) (ExecOutput, error) {
			gotCommand = append([]string(nil), spec.Command...)
			data, err := os.ReadFile(filepath.Join(spec.WorkspaceDir, "todo", "list.go"))
			if err != nil {
				t.Fatalf("read edited file: %v", err)
			}
			if !strings.Contains(string(data), "aggregate.Next(l, ListCreated, title)") {
				t.Fatalf("workspace did not contain edited file content")
			}
			return ExecOutput{Stdout: "Groceries", Duration: 15 * time.Millisecond}, nil
		}},
		TempDir: t.TempDir(),
	}

	content, err := os.ReadFile(filepath.Join(testLessonsRoot(t), "quick-start", "solution", "todo", "list.go"))
	if err != nil {
		t.Fatalf("read solution: %v", err)
	}

	result, err := service.Execute(context.Background(), ExecuteRequest{
		LessonID: "quick-start",
		Action:   ActionRun,
		Files: []FileUpdate{{
			Path:    "todo/list.go",
			Content: string(content),
		}},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if result.Status != "passed" {
		t.Fatalf("unexpected status: %s", result.Status)
	}
	wantCommand := lessons["quick-start"].Manifest.Run.Command
	if !reflect.DeepEqual(gotCommand, wantCommand) {
		t.Fatalf("run command = %v, want %v", gotCommand, wantCommand)
	}
}

func TestServiceRejectsLockedFileEdits(t *testing.T) {
	lessons, err := LoadLessons(testLessonsRoot(t))
	if err != nil {
		t.Fatalf("load lessons: %v", err)
	}

	service := Service{
		Lessons: lessons,
		Exec: stubExecutor{run: func(_ context.Context, _ ExecSpec) (ExecOutput, error) {
			return ExecOutput{}, nil
		}},
		TempDir: t.TempDir(),
	}

	_, err = service.Execute(context.Background(), ExecuteRequest{
		LessonID: "quick-start",
		Action:   ActionRun,
		Files: []FileUpdate{{
			Path:    "cmd/app/main.go",
			Content: "package main",
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "not editable") {
		t.Fatalf("expected locked file error, got %v", err)
	}
}

func TestServicePropagatesTimeout(t *testing.T) {
	lessons, err := LoadLessons(testLessonsRoot(t))
	if err != nil {
		t.Fatalf("load lessons: %v", err)
	}

	service := Service{
		Lessons: lessons,
		Exec: stubExecutor{run: func(ctx context.Context, _ ExecSpec) (ExecOutput, error) {
			<-ctx.Done()
			return ExecOutput{}, ctx.Err()
		}},
		TempDir: t.TempDir(),
		Timeout: 10 * time.Millisecond,
	}

	_, err = service.Execute(context.Background(), ExecuteRequest{LessonID: "quick-start", Action: ActionRun})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}
