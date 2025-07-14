package nats

import "strings"

var replacer = strings.NewReplacer(
	".", "_",
	">", "_",
	"*", "_",
	// " ", "_",
)

func subscribeSubject(userProvidedSubject, event string) string {
	if event == "*" {
		return ">"
	}
	return userProvidedSubject
}
