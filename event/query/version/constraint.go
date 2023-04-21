// Package version provides version constraints for queries.
package version

// Constraints are the version constraints for an event query. Methods of
// Constraints that return non-nil filters must all be fulfilled by an event to
// be included in the query result. If a filter allows multiple values, the
// event must match at least one of the values.
type Constraints interface {
	// Exact returns the exact versions to query for.
	Exact() []int

	// Ranges returns the version ranges to query for.
	Ranges() []Range

	// Min returns the minimum allowed versions to query for.
	Min() []int

	// Max returns the maximu allowed versions to query for.
	Max() []int
}

// A Option defines a version constraint.
type Option func(*constraints)

// Range is a version range.
type Range [2]int

type constraints struct {
	exact  []int
	ranges []Range
	min    []int
	max    []int
}

// Merge merges mutliple Constraints into one.
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
			Min(c.Min()...),
			Max(c.Max()...),
		)
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

// Exact returns a Constraint that only allows the exact versions v.
func Exact(v ...int) Option {
	return func(c *constraints) {
	L:
		for _, v := range v {
			for _, v2 := range c.exact {
				if v == v2 {
					continue L
				}
			}
			c.exact = append(c.exact, v)
		}
	}
}

// InRange returns a Constraint that only allows versions in the Ranges r.
func InRange(r ...Range) Option {
	return func(c *constraints) {
	L:
		for _, r := range r {
			for _, r2 := range c.ranges {
				if r == r2 {
					continue L
				}
			}
			c.ranges = append(c.ranges, r)
		}
	}
}

// Min returns a Constraint that only allows versions that are >= least one of v.
func Min(v ...int) Option {
	return func(c *constraints) {
	L:
		for _, v := range v {
			for _, v2 := range c.min {
				if v == v2 {
					continue L
				}
			}
			c.min = append(c.min, v)
		}
	}
}

// Max returns a Constraint that only allows versions that are <= at least one of v.
func Max(v ...int) Option {
	return func(c *constraints) {
	L:
		for _, v := range v {
			for _, v2 := range c.max {
				if v == v2 {
					continue L
				}
			}
			c.max = append(c.max, v)
		}
	}
}

// Includes determines if the Constraints c includes all of v.
func Includes(c Constraints, v ...int) bool {
	if violatesExact(c.Exact(), v...) {
		return false
	}

	if violatesRanges(c.Ranges(), v...) {
		return false
	}

	if violatesMin(c.Min(), v...) {
		return false
	}

	if violatesMax(c.Max(), v...) {
		return false
	}

	return true
}

// Exact returns a Constraint that only allows the exact versions v.
func (c constraints) Exact() []int {
	return c.exact
}

// Ranges is an interface that defines the version ranges to query for an event 
// in a Constraints object. It is a method of the Constraints interface and 
// returns a slice of Range values. A Range is a type that represents a version 
// range as an array of two integers. The Start method returns the start of the 
// Range and the End method returns the end. The Includes method checks if a 
// given integer value is within the range.
func (c constraints) Ranges() []Range {
	return c.ranges
}

// Min returns a Constraint that only allows versions that are greater than or 
// equal to at least one of the provided version numbers. It is a function that 
// takes integer arguments and returns an Option function that modifies a 
// Constraints object.
func (c constraints) Min() []int {
	return c.min
}

// Max returns a Constraint that only allows versions that are <= at least one 
// of v. [Max] is an Option function that takes a pointer to constraints struct 
// as input and modifies its max field to include the provided version(s). If a 
// version is already included in the field, it will not be added again. The 
// function can be used with other Options to create complex Constraints using 
// Merge and Filter functions.
func (c constraints) Max() []int {
	return c.max
}

// Start returns the start of the Range (r[0]).
func (r Range) Start() int {
	return r[0]
}

// End returns the end of the Range (r[1]).
func (r Range) End() int {
	return r[1]
}

// Includes returns true if v is within the Range r.
func (r Range) Includes(v int) bool {
	return v >= r[0] && v <= r[1]
}

func violatesExact(exact []int, vs ...int) bool {
	if len(exact) == 0 {
		return false
	}
	for _, v := range vs {
		for _, v2 := range exact {
			if v == v2 {
				return false
			}
		}
	}
	return true
}

func violatesRanges(ranges []Range, vs ...int) bool {
	if len(ranges) == 0 {
		return false
	}
	for _, v := range vs {
		for _, r := range ranges {
			if r.Includes(v) {
				return false
			}
		}
	}
	return true
}

func violatesMin(min []int, vs ...int) bool {
	if len(min) == 0 {
		return false
	}
	for _, v := range vs {
		for _, m := range min {
			if v >= m {
				return false
			}
		}
	}
	return true
}

func violatesMax(max []int, vs ...int) bool {
	if len(max) == 0 {
		return false
	}
	for _, v := range vs {
		for _, m := range max {
			if v <= m {
				return false
			}
		}
	}
	return true
}
