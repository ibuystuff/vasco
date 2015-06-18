package cache

import "strings"

// this is a chainable helper to manage a set of unique items
type StringSet struct {
	isNegative bool
	content    map[string]bool
}

func NewStringSet() *StringSet {
	ss := new(StringSet)
	ss.content = make(map[string]bool)
	return ss
}

func (ss *StringSet) Negate() *StringSet {
	ss.isNegative = !ss.isNegative
	return ss
}

func (ss *StringSet) Add(s string) *StringSet {
	ss.content[s] = true
	return ss
}

func (ss *StringSet) AddMultiple(sa []string) *StringSet {
	for _, s := range sa {
		ss.Add(s)
	}
	return ss
}

// symmetric difference -- intersects two sets and returns a new set
// abc & cde == c
// abc & !cde == ab
// !abc & cde == de
// !abc & !cde == !abcde

// this helper can only cope with ss2 possibly being negative
func (ss1 *StringSet) intersection(ss2 *StringSet) *StringSet {
	// if we have 2 positive sets we can optimize for length
	if !ss2.isNegative {
		l1 := len(ss1.content)
		l2 := len(ss2.content)
		if l2 < l1 {
			ss2, ss1 = ss1, ss2
		}
	}

	intersection := NewStringSet()
	for k := range ss1.content {
		if _, ok := ss2.content[k]; ok != ss2.isNegative {
			intersection.Add(k)
		}
	}
	return intersection
}

func (ss1 *StringSet) Intersection(ss2 *StringSet) *StringSet {
	var r *StringSet
	switch {
	case !ss1.isNegative && !ss2.isNegative:
		r = ss1.intersection(ss2)
	case !ss1.isNegative && ss2.isNegative:
		r = ss1.intersection(ss2)
	case ss1.isNegative && !ss2.isNegative:
		r = ss2.intersection(ss1)
	case ss1.isNegative && ss2.isNegative:
		r = ss1.Union(ss2).Negate()
	}
	return r
}

// creates the union (sum) of the two sets
func (ss1 *StringSet) Union(ss2 *StringSet) *StringSet {
	union := NewStringSet()
	for k := range ss1.content {
		union.Add(k)
	}
	for k := range ss2.content {
		union.Add(k)
	}
	return union
}

// assymetric set difference -- returns a new set
func (ss1 *StringSet) Subtract(ss2 *StringSet) *StringSet {
	difference := NewStringSet()
	for k := range ss1.content {
		if _, ok := ss2.content[k]; !ok {
			difference.Add(k)
		}
	}
	return difference
}

func (ss *StringSet) Delete(s string) *StringSet {
	if _, ok := ss.content[s]; ok {
		delete(ss.content, s)
	}
	return ss
}

func (ss *StringSet) DeleteMultiple(sa []string) *StringSet {
	for _, s := range sa {
		ss.Delete(s)
	}
	return ss
}

func (ss *StringSet) Strings() []string {
	a := make([]string, 0, len(ss.content))
	for s := range ss.content {
		a = append(a, s)
	}
	return a
}

func (ss *StringSet) Join(sep string) string {
	return strings.Join(ss.Strings(), sep)
}

func (ss *StringSet) WrappedJoin(prefix string, sep string, suffix string) string {
	return prefix + strings.Join(ss.Strings(), sep) + suffix
}

func (ss *StringSet) Length() int {
	return len(ss.content)
}
