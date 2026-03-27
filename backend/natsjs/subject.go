package natsjs

import (
	"strings"

	"github.com/google/uuid"
)

const (
	// subjectSentinel is used for missing aggregate name or ID in subjects.
	subjectSentinel = "_"
)

// encodeSubject builds a NATS subject for publishing an event.
// Format: <ns>.<aggregateName>.<aggregateID>.<eventName>
func encodeSubject(ns, eventName, aggName string, aggID uuid.UUID) string {
	var b strings.Builder
	b.WriteString(ns)
	b.WriteByte('.')
	if aggName == "" {
		b.WriteString(subjectSentinel)
	} else {
		b.WriteString(escapeSubjectToken(aggName))
	}
	b.WriteByte('.')
	if aggID == uuid.Nil {
		b.WriteString(subjectSentinel)
	} else {
		b.WriteString(aggID.String())
	}
	b.WriteByte('.')
	b.WriteString(escapeSubjectToken(eventName))
	return b.String()
}

// aggStreamSubjects returns the subject patterns for a per-aggregate-type stream.
func aggStreamSubjects(ns, aggName string) []string {
	return []string{ns + "." + escapeSubjectToken(aggName) + ".>"}
}

// noAggStreamSubjects returns the subject patterns for the non-aggregate stream.
func noAggStreamSubjects(ns string) []string {
	return []string{ns + "." + subjectSentinel + ".>"}
}

// aggregateFilterSubject builds a subject filter for a specific aggregate name
// and optional ID within an aggregate stream.
// With ID:    <ns>.<aggName>.<aggID>.>
// Without ID: <ns>.<aggName>.>  (matches all instances)
func aggregateFilterSubject(ns, aggName string, aggID uuid.UUID) string {
	var b strings.Builder
	b.WriteString(ns)
	b.WriteByte('.')
	b.WriteString(escapeSubjectToken(aggName))
	b.WriteByte('.')
	if aggID == uuid.Nil {
		// All instances of this aggregate type.
		b.WriteByte('>')
	} else {
		b.WriteString(aggID.String())
		b.WriteString(".>")
	}
	return b.String()
}

// escapeSubjectToken replaces characters that are special in NATS subjects.
func escapeSubjectToken(s string) string {
	r := strings.NewReplacer(".", "_", "*", "_", ">", "_", " ", "_")
	return r.Replace(s)
}
