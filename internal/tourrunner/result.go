package tourrunner

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"time"
)

type CheckResult struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type RunResult struct {
	Status        string        `json:"status"`
	Stdout        string        `json:"stdout,omitempty"`
	Stderr        string        `json:"stderr,omitempty"`
	DurationMs    int64         `json:"durationMs"`
	CompileErrors []string      `json:"compileErrors,omitempty"`
	Checks        []CheckResult `json:"checks,omitempty"`
}

type testEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Output  string `json:"Output"`
}

func NewRunResult(stdout, stderr string, duration time.Duration, err error) RunResult {
	result := RunResult{
		Status:     "passed",
		Stdout:     strings.TrimSpace(stdout),
		Stderr:     strings.TrimSpace(stderr),
		DurationMs: duration.Milliseconds(),
	}

	if err != nil {
		result.Status = "failed"
		result.CompileErrors = parseCompileErrors(stderr)
	}

	return result
}

func NewCheckResult(stdout, stderr string, duration time.Duration, err error) RunResult {
	result := RunResult{
		Status:     "passed",
		Stdout:     strings.TrimSpace(stdout),
		Stderr:     strings.TrimSpace(stderr),
		DurationMs: duration.Milliseconds(),
		Checks:     parseGoTestJSON(stdout),
	}

	if err != nil {
		result.Status = "failed"
		result.CompileErrors = parseCompileErrors(stderr)
		if len(result.Checks) == 0 {
			result.Checks = []CheckResult{{
				ID:      "check",
				Label:   "Lesson checks",
				Status:  "failed",
				Message: firstLine(stderr),
			}}
		}
	}

	if len(result.Checks) == 0 {
		result.Checks = []CheckResult{{
			ID:      "check",
			Label:   "Lesson checks",
			Status:  result.Status,
			Message: defaultCheckMessage(result.Status),
		}}
	}

	return result
}

func parseGoTestJSON(stdout string) []CheckResult {
	checks := make(map[string]CheckResult)
	var order []string

	scanner := bufio.NewScanner(strings.NewReader(stdout))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 || line[0] != '{' {
			continue
		}

		var event testEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		if event.Test == "" {
			continue
		}

		check := checks[event.Test]
		if check.ID == "" {
			check = CheckResult{ID: event.Test, Label: event.Test, Status: "pending"}
			order = append(order, event.Test)
		}

		switch event.Action {
		case "pass":
			check.Status = "passed"
			if check.Message == "" {
				check.Message = "Check passed"
			}
		case "fail":
			check.Status = "failed"
			if check.Message == "" {
				check.Message = "Check failed"
			}
		case "output":
			msg := strings.TrimSpace(event.Output)
			if msg != "" && !strings.HasPrefix(msg, "===") && !strings.HasPrefix(msg, "---") {
				check.Message = msg
			}
		}

		checks[event.Test] = check
	}

	results := make([]CheckResult, 0, len(order))
	for _, id := range order {
		results = append(results, checks[id])
	}
	return results
}

func parseCompileErrors(stderr string) []string {
	var errs []string
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		errs = append(errs, line)
		if len(errs) == 5 {
			break
		}
	}
	return errs
}

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func defaultCheckMessage(status string) string {
	if status == "passed" {
		return "All checks passed"
	}
	return "Fix the lesson checks and try again"
}
