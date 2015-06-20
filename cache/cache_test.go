package cache

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/AchievementNetwork/stringset"
	"github.com/stretchr/testify/assert"
)

// we want to support using the same tests for a variety of implementations
// of the datastore, so we'll declare the datastore as a global
var c Cache

// this is how many keys we try when we do tests of N keys
const N = 12

func TestMain(m *testing.M) {
	// always starts wiped clean
	c = NewLocalCache()
	defer c.Close()

	memResult := m.Run()

	os.Exit(memResult)
}

func TestTest(t *testing.T) {
	t.Logf("Now testing %T\n", c)
}

func TestBasicSetGet(t *testing.T) {
	err := c.Set("key", "value")
	assert.Nil(t, err)
	v, err := c.Get("key")
	assert.Nil(t, err)
	assert.Equal(t, "value", v)
}

func TestOverwritingKey(t *testing.T) {
	err := c.Set("key", "newvalue")
	v, err := c.Get("key")
	assert.Nil(t, err)
	assert.Equal(t, "newvalue", v)
}

func TestGetFail(t *testing.T) {
	v, err := c.Get("badkey")
	assert.NotNil(t, err)
	assert.Empty(t, v)
}

func TestDelete(t *testing.T) {
	err := c.Delete("key")
	assert.Nil(t, err)
	v, err := c.Get("key")
	assert.NotNil(t, err)
	assert.Empty(t, v)
	err = c.Delete("key")
	assert.NotNil(t, err)
}

func TestMultipleKeys(t *testing.T) {
	keys := make([]string, 0)
	values := make([]string, 0)
	for i := 0; i < N; i++ {
		k := fmt.Sprintf("key%d", i)
		v := fmt.Sprintf("value%d", i)
		keys = append(keys, k)
		values = append(values, v)
		c.Set(k, v)
	}

	for k2 := range keys {
		v, err := c.Get(keys[k2])
		assert.Nil(t, err)
		assert.Equal(t, values[k2], v)
		c.Delete(keys[k2])
	}
}

// we know this tests both Expire and ExpireAt because of the way the code is written
// You can skip this test by doing "go test -test.short"
func TestExpire(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	// we wait until we're in the first tenth part of a second
	// so that we have some predictability over when things expire
	for {
		t := time.Time.Nanosecond(time.Now())
		if t < 100000000 {
			break
		}
		time.Sleep(90 * time.Millisecond)
	}

	keys := make([]string, 0)
	values := make([]string, 0)
	for i := 0; i < N; i++ {
		k := fmt.Sprintf("key%d", i)
		v := fmt.Sprintf("value%d", i)
		keys = append(keys, k)
		values = append(values, v)
		c.Set(k, v)
		c.Expire(k, i%3) // 0, 1 and 2 seconds in the future - zero second keys expire immediately
	}

	found := 0
	for k2 := range keys {
		if _, err := c.Get(keys[k2]); err == nil {
			found += 1
		}
	}
	assert.Equal(t, 2*N/3, found, "Zero expiration time failed.")
	time.Sleep(1000 * time.Millisecond)

	found = 0
	for k2 := range keys {
		if _, err := c.Get(keys[k2]); err == nil {
			found += 1
		}
	}
	assert.Equal(t, N/3, found, "1 second expiration time failed.")
	time.Sleep(1000 * time.Millisecond)

}

func TestSAddSGet(t *testing.T) {
	values := []string{"a", "b", "c", "d", "e"}
	err := c.SAdd("skey", values...)
	assert.Nil(t, err)
	result, err := c.SGet("skey")
	assert.Nil(t, err)
	checkEquivalence(t, values, result)
}

func TestMultipleAdd(t *testing.T) {
	values := []string{"a", "b", "c", "d", "e"}
	err := c.SAdd("skey", "a", "b", "c")
	assert.Nil(t, err)
	err = c.SAdd("skey", "c", "d", "e")
	assert.Nil(t, err)
	result, err := c.SGet("skey")
	assert.Nil(t, err)
	checkEquivalence(t, values, result)
}

func TestSCount(t *testing.T) {
	n, err := c.SCount("skey")
	assert.Nil(t, err)
	assert.Equal(t, 5, n)
	n, err = c.SCount("skeynotfound")
	assert.NotNil(t, err)
	assert.Equal(t, 0, n)
}

func TestSRemove(t *testing.T) {
	err := c.SRemove("skey", "a", "b", "d")
	assert.Nil(t, err)
	result, err := c.SGet("skey")
	assert.Nil(t, err)
	n, _ := c.SCount("skey")
	assert.Equal(t, 2, n)
	checkEquivalence(t, []string{"c", "e"}, result)
	err = c.SRemove("skey", "a", "c", "e")
	assert.Nil(t, err)
	result, err = c.SGet("skey")
	assert.NotNil(t, err) // skey should have been deleted by previous call
}

func TestSRandom(t *testing.T) {
	err := c.SAdd("skey", "a", "b", "c", "d", "e")
	assert.Nil(t, err)

	// generate 1000 random results and make sure they're distributed evenly.
	// This test can fail every once in a while, but not often enough to be a problem.
	// If I'm wrong about that, change the 30 to 35.
	results := map[string]int{"a": 0, "b": 0, "c": 0, "d": 0, "e": 0}
	for i := 0; i < 1000; i++ {
		s, err := c.SRandMember("skey")
		assert.Nil(t, err)
		results[s] += 1
	}
	for _, v := range results {
		assert.InDelta(t, 200, v, 30)
	}
}

func TestZAddZRange(t *testing.T) {
	err := c.ZAdd("key", 15, "a")
	assert.Nil(t, err)
	err = c.ZAdd("key", 25, "b")
	assert.Nil(t, err)
	err = c.ZAdd("key", 5, "c")
	assert.Nil(t, err)
	err = c.ZAdd("key", 45, "d")
	assert.Nil(t, err)
	err = c.ZAdd("key", 35, "e")
	assert.Nil(t, err)

	values := []string{"c", "a", "b", "e", "d"}
	results, err := c.ZRange("key", 0, -1)
	assert.Nil(t, err)
	assert.Equal(t, values, results)

	results, err = c.ZRange("key", 0, 2)
	assert.Nil(t, err)
	assert.Equal(t, []string{"c", "a", "b"}, results)

	results, err = c.ZRange("key", 3, 3)
	assert.Nil(t, err)
	assert.Equal(t, []string{"e"}, results)

	results, err = c.ZRange("key", -2, -1)
	assert.Nil(t, err)
	assert.Equal(t, []string{"e", "d"}, results)

	results, err = c.ZRange("key", -20, -10)
	assert.Nil(t, err)
	assert.Equal(t, []string{}, results)

	results, err = c.ZRange("key", 20, 10)
	assert.Nil(t, err)
	assert.Equal(t, []string{}, results)

	// The redis documentation is unclear on what this test should return;
	// when we implement redis, if some of these tests fail with the straightforward
	// implementation, we should change the tests and fix the memory cache.
	results, err = c.ZRange("key", 10, 20)
	assert.Nil(t, err)
	assert.Equal(t, []string{}, results)

	results, err = c.ZRange("key", 4, 20)
	assert.Nil(t, err)
	assert.Equal(t, []string{"d"}, results)
}

func TestZAddAgain(t *testing.T) {
	err := c.ZAdd("key", 55, "a")
	assert.Nil(t, err)

	values := []string{"c", "b", "e", "d", "a"}
	results, err := c.ZRange("key", 0, -1)
	assert.Nil(t, err)
	assert.Equal(t, values, results)
}

// This performs an equivalence test for two string slices
func checkEquivalence(t *testing.T, a []string, b []string) {
	ssa := stringset.New().Add(a...)
	ssb := stringset.New().Add(b...)
	assert.True(t, ssa.Equals(ssb))
}
