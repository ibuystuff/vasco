package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringSetBasic(t *testing.T) {
	ss := NewStringSet()
	ss.Add("this")
	ss.Add("is")
	ss.Add("a")
	ss.Add("test")
	CheckTagEquivalence(t, ss.Strings(), []string{"this", "is", "a", "test"})
	ss.Delete("this")
	CheckTagEquivalence(t, ss.Strings(), []string{"is", "a", "test"})
	ss.DeleteMultiple([]string{"is", "nothing"})
	CheckTagEquivalence(t, ss.Strings(), []string{"a", "test"})
	ss.AddMultiple([]string{"this", "is", "is", "a", "test"})
	CheckTagEquivalence(t, ss.Strings(), []string{"this", "is", "a", "test"})
}

func TestStringSetOperations(t *testing.T) {
	ss1 := NewStringSet()
	ss1.AddMultiple([]string{"this", "is", "a", "test"})
	ss2 := NewStringSet()
	ss2.AddMultiple([]string{"this", "was", "an", "interesting", "test"})

	inter := ss1.Intersection(ss2)
	CheckTagEquivalence(t, inter.Strings(), []string{"this", "test"})
	union := ss1.Union(ss2)
	CheckTagEquivalence(t, union.Strings(), []string{"this", "was", "an", "interesting", "test", "is", "a"})
	s1 := ss1.Subtract(ss2)
	CheckTagEquivalence(t, s1.Strings(), []string{"is", "a"})
	s2 := ss2.Subtract(ss1)
	CheckTagEquivalence(t, s2.Strings(), []string{"was", "an", "interesting"})
}

func TestStringSetNegate2(t *testing.T) {
	ss1 := NewStringSet().AddMultiple([]string{"a", "b", "c"})
	ss2 := NewStringSet().AddMultiple([]string{"c", "d", "e"}).Negate()
	pn := ss1.Intersection(ss2)
	CheckTagEquivalence(t, pn.Strings(), []string{"a", "b"})
}

func TestStringSetNegate1(t *testing.T) {
	ss1 := NewStringSet().AddMultiple([]string{"a", "b", "c"}).Negate()
	ss2 := NewStringSet().AddMultiple([]string{"c", "d", "e"})
	pn := ss1.Intersection(ss2)
	CheckTagEquivalence(t, pn.Strings(), []string{"d", "e"})
}

func TestStringSetNegate12(t *testing.T) {
	ss1 := NewStringSet().AddMultiple([]string{"a", "b", "c"}).Negate()
	ss2 := NewStringSet().AddMultiple([]string{"c", "d", "e"}).Negate()
	pn := ss1.Intersection(ss2)
	CheckTagEquivalence(t, pn.Strings(), []string{"a", "b", "c", "d", "e"})
}

func CheckTagEquivalence(t *testing.T, a []string, b []string) {
	// make a copy of a in c
	c := make([]string, len(a))
	copy(c, a)
	// look at every item in b and remove it from c
	for _, v := range b {
		found := false
		for i, v2 := range c {
			if v == v2 {
				// remove it from c by copying the last item over the found item and shortening the slice
				c[i], c = c[len(c)-1], c[:len(c)-1]
				found = true
				break
			}
		}
		assert.True(t, found, "Tag '%v' not found in first list (%v)", v, c)
	}
	assert.Empty(t, c, "Extra tags found in first list: '%v'", c)
}
