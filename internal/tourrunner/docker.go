package tourrunner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Executor interface {
	Run(ctx context.Context, spec ExecSpec) (ExecOutput, error)
}

type ExecSpec struct {
	WorkspaceDir string
	Command      []string
	Timeout      time.Duration
}

type ExecOutput struct {
	Stdout   string
	Stderr   string
	Duration time.Duration
}

type DockerExecutor struct {
	DockerBin string
	Image     string
	RepoRoot  string
	Memory    string
	CPUs      string
	PIDs      string
}

func (e DockerExecutor) Run(ctx context.Context, spec ExecSpec) (ExecOutput, error) {
	start := time.Now()
	if len(spec.Command) == 0 {
		return ExecOutput{}, fmt.Errorf("missing command")
	}
	command := normalizeCommand(spec.Command)

	args := []string{
		"run", "--rm",
		"--network", "none",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges:true",
		"--memory", valueOrDefault(e.Memory, "1g"),
		"--cpus", valueOrDefault(e.CPUs, "1"),
		"--pids-limit", valueOrDefault(e.PIDs, "128"),
		"--read-only",
		"--tmpfs", "/tmp:rw,nosuid,size=1g",
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/workspace", spec.WorkspaceDir),
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/opt/goes,readonly", e.RepoRoot),
		"-w", "/workspace",
		"-e", "PATH=/go/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"-e", "TMPDIR=/workspace/.tmp",
		"-e", "GOCACHE=/tmp/go-cache",
		"-e", "GOMODCACHE=/go/pkg/mod",
		"-e", "GOFLAGS=-mod=mod",
		"-e", "GOSUMDB=off",
		valueOrDefault(e.Image, "golang:1.24.1-bookworm"),
		"sh", "-lc", shellQuote(command),
	}

	cmd := exec.CommandContext(ctx, valueOrDefault(e.DockerBin, "docker"), args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return ExecOutput{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: time.Since(start),
	}, err
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func shellQuote(command []string) string {
	parts := make([]string, 0, len(command))
	for _, part := range command {
		parts = append(parts, fmt.Sprintf("'%s'", strings.ReplaceAll(part, "'", `'"'"'`)))
	}
	return strings.Join(parts, " ")
}

func normalizeCommand(command []string) []string {
	if len(command) == 0 {
		return nil
	}
	normalized := append([]string(nil), command...)
	if normalized[0] == "go" {
		normalized[0] = "/usr/local/go/bin/go"
	}
	return normalized
}

func ResolveRepoRoot(path string) (string, error) {
	if path == "" {
		path = "."
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve repo root: %w", err)
	}
	return abs, nil
}
