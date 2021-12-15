package nats

import "strings"

// "." is a reserved character by NATS for subjects, queue groups etc.
func replaceDots(s string) string {
	return strings.ReplaceAll(s, ".", "_")
}
