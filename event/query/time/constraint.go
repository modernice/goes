// Package time provides time constraints for queries.
package time

import "time"

// Constraints provides the different time constraints for querying events. An
// event.Store that uses Constraints should combine the different types of
// constraints with a logical "AND" and the different values for a constraint
// with a logical "OR".
type Constraints interface {
	// Exact returns the exact times to query for.
	Exact() []time.Time

	// Ranges returns the time ranges to query for.
	Ranges() []Range

	// Min returns the minimum allowed time to query for. Zero time means this
	// filter is disabled
	Min() time.Time

	// Max returns the maximum allowed time to query for. Zero time means this
	// filter is disabled
	Max() time.Time
}

// A Option is an option for Constraints.
type Option func(*constraints)

// Range is a time range.
type Range [2]time.Time

type constraints struct {
	exact  []time.Time
	ranges []Range
	min    time.Time
	max    time.Time
}

// Merge merges the provided Constraints into one.
func Merge(constraints ...Constraints) Constraints {
	return Filter(DryMerge(constraints...)...)
}

// DryMerge returns the Options to merge the provided Constraints.
func DryMerge(constraints ...Constraints) []Option {
	var opts []Option
	for _, c := range constraints {
		opts = append(
			opts,
			Exact(c.Exact()...),
			InRange(c.Ranges()...),
		)

		if t := c.Min(); !t.IsZero() {
			opts = append(opts, Min(t))
		}

		if t := c.Max(); !t.IsZero() {
			opts = append(opts, Max(t))
		}
	}
	return opts
}

// Filter returns Constraints from the given Constraint opts.
func Filter(opts ...Option) Constraints {
	var c constraints
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// Exact returns a Constraint that only allows the exact times t.
func Exact(t ...time.Time) Option {
	return func(c *constraints) {
		c.exact = append(c.exact, t...)
	}
}

// InRange returns a Constraint that only allows times in the Ranges r.
func InRange(r ...Range) Option {
	return func(c *constraints) {
		c.ranges = append(c.ranges, r...)
	}
}

// Before returns a Constraint that only allows times that are before at least one of v.
func Before(t time.Time) Option {
	return Max(t.Add(-time.Nanosecond))
}

// After returns a Constraint that only allows times that are after at least one of v.
func After(t time.Time) Option {
	return Min(t.Add(time.Nanosecond))
}

// Min returns a Constraint that only allows times that are >= at least one of v.
func Min(min time.Time) Option {
	return func(c *constraints) {
		c.min = min
	}
}

// Max returns a Constraint that only allows times that are <= at least one of v.
func Max(max time.Time) Option {
	return func(c *constraints) {
		c.max = max
	}
}

// Includes determines if the Constraints c include all of t.
func Includes(c Constraints, t ...time.Time) bool {
	if violatesExact(c.Exact(), t...) {
		return false
	}

	if violatesRanges(c.Ranges(), t...) {
		return false
	}

	if violatesMin(c.Min(), t...) {
		return false
	}

	if violatesMax(c.Max(), t...) {
		return false
	}

	return true
}

func (c constraints) Exact() []time.Time {
	return c.exact
}

func (c constraints) Ranges() []Range {
	return c.ranges
}

func (c constraints) Min() time.Time {
	return c.min
}

func (c constraints) Max() time.Time {
	return c.max
}

// Start returns the start of the range (r[0]):
func (r Range) Start() time.Time {
	return r[0]
}

// End returns the end of the range (r[1]):
func (r Range) End() time.Time {
	return r[1]
}

// Includes returns true if t is within the Range r.
func (r Range) Includes(t time.Time) bool {
	return t.Equal(r[0]) || t.Equal(r[1]) || (t.After(r[0]) && t.Before(r[1]))
}

func violatesExact(exact []time.Time, t ...time.Time) bool {
	if len(exact) == 0 {
		return false
	}
	for _, t := range t {
		for _, t2 := range exact {
			if t2.Equal(t) {
				return false
			}
		}
	}
	return true
}

func violatesRanges(ranges []Range, t ...time.Time) bool {
	if len(ranges) == 0 {
		return false
	}
	for _, t := range t {
		for _, r := range ranges {
			if r.Includes(t) {
				return false
			}
		}
	}
	return true
}

func violatesMin(min time.Time, t ...time.Time) bool {
	if min.IsZero() {
		return false
	}
	for _, t := range t {
		if t.After(min.Add(-time.Nanosecond)) {
			return false
		}
	}
	return true
}

func violatesMax(max time.Time, t ...time.Time) bool {
	if max.IsZero() {
		return false
	}
	for _, t := range t {
		if t.Before(max.Add(time.Nanosecond)) {
			return false
		}
	}
	return true
}
