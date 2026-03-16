package tourrunner

import "testing"

func TestNewCheckResultParsesGoTestJSON(t *testing.T) {
	stdout := "{\"Action\":\"run\",\"Test\":\"TestLesson\"}\n" +
		"{\"Action\":\"output\",\"Test\":\"TestLesson\",\"Output\":\"expected renamed product\\n\"}\n" +
		"{\"Action\":\"fail\",\"Test\":\"TestLesson\"}\n"

	result := NewCheckResult(stdout, "", 0, assertErr{})
	if result.Status != "failed" {
		t.Fatalf("status = %s, want failed", result.Status)
	}
	if len(result.Checks) != 1 {
		t.Fatalf("checks = %d, want 1", len(result.Checks))
	}
	if result.Checks[0].Message != "expected renamed product" {
		t.Fatalf("message = %q", result.Checks[0].Message)
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "failed" }
