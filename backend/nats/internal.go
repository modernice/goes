package nats

import "strings"

var replacer = strings.NewReplacer(
	".", "_",
	">", "_",
	"*", "_",
	// " ", "_",
)

func replaceDots(s string) string {
	return strings.ReplaceAll(s, ".", "_")
}
