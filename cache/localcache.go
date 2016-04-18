package cache

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AchievementNetwork/stringset"
)

type cacheValue struct {
	value string
	exp   int64
}

// an in-memory implementation of the Cache
// great for testing or for deploying on a small scale
type LocalCache struct {
	values     map[string]cacheValue
	valuemutex sync.RWMutex
	sets       map[string]*stringset.StringSet
	setmutex   sync.RWMutex
}

func NewLocalCache() *LocalCache {
	c := LocalCache{
		values: make(map[string]cacheValue),
		sets:   make(map[string]*stringset.StringSet),
	}
	return &c
}

func (c *LocalCache) Close() {
}

func (c *LocalCache) Set(key string, value string) (err error) {
	c.valuemutex.Lock()
	c.values[key] = cacheValue{value: value, exp: 0}
	c.valuemutex.Unlock()
	err = nil
	return
}

func (c *LocalCache) getUnexpired(key string, now int64) (item cacheValue, err error) {
	c.valuemutex.RLock()
	v, ok := c.values[key]
	c.valuemutex.RUnlock()

	if ok {
		// fmt.Printf("%s %d %d\n", key, now, v.exp)
		if v.exp == 0 || now < v.exp {
			item = v
			return
		}
		c.valuemutex.Lock()
		delete(c.values, key) // key has expired
		c.valuemutex.Unlock()
	}

	err = errors.New("Key not found")
	return
}

func (c *LocalCache) Get(key string) (value string, err error) {
	if item, e := c.getUnexpired(key, time.Time.Unix(time.Now())); e == nil {
		value = item.value
	} else {
		err = e
	}
	return
}

func (c *LocalCache) Delete(key string) (err error) {
	c.valuemutex.RLock()
	_, ok := c.values[key]
	c.valuemutex.RUnlock()

	if ok {
		c.valuemutex.Lock()
		delete(c.values, key) // key has expired
		c.valuemutex.Unlock()
		return
	}
	err = errors.New("Key not found")
	return
}

func (c *LocalCache) Expire(key string, seconds int) (err error) {
	exptime := time.Time.Unix(time.Now()) + int64(seconds)
	return c.ExpireAt(key, exptime)
}

func (c *LocalCache) ExpireAt(key string, timestamp int64) (err error) {
	now := time.Time.Unix(time.Now())
	if item, err := c.getUnexpired(key, now); err == nil {
		item.exp = timestamp
		c.values[key] = item
	}
	return
}

func (c *LocalCache) SAdd(key string, values ...string) (err error) {
	c.setmutex.Lock()
	s, ok := c.sets[key]

	if !ok {
		s = stringset.New()
	}
	s.Add(values...)
	c.sets[key] = s
	c.setmutex.Unlock()
	return
}

func (c *LocalCache) SGet(key string) (values []string, err error) {
	c.setmutex.RLock()
	s, ok := c.sets[key]

	if ok {
		values = s.Strings()
	} else {
		err = errors.New("Key not found")
	}
	c.setmutex.RUnlock()
	return
}

func (c *LocalCache) SRemove(key string, values ...string) (err error) {
	c.setmutex.Lock()
	s, ok := c.sets[key]

	if ok {
		s.Delete(values...)
		// if we've removed the last item, delete the key
		if s.Length() == 0 {
			delete(c.sets, key)
		} else {
			c.sets[key] = s
		}
	} else {
		err = errors.New("Key not found")
	}
	c.setmutex.Unlock()
	return
}

func (c *LocalCache) SCount(key string) (count int, err error) {
	c.setmutex.RLock()
	s, ok := c.sets[key]

	if ok {
		count = s.Length()
	} else {
		err = errors.New("Key not found")
	}
	c.setmutex.RUnlock()
	return
}

func (c *LocalCache) SRandMember(key string) (value string, err error) {
	c.setmutex.RLock()
	s, ok := c.sets[key]

	if ok {
		all := s.Strings()
		r := rand.Intn(len(all))
		value = all[r]
	} else {
		err = errors.New("Key not found")
	}
	c.setmutex.RUnlock()
	return
}

// warning -- this stuff is worse than O(n) so if we start managing big lists, this needs a rewrite
// or to use Redis.
func (c *LocalCache) ZAdd(key string, score int, value string) (err error) {
	newvalue := fmt.Sprintf("%d:%s", score, value)
	c.ZRem(key, value) // in case it already exists
	return c.SAdd("Z"+key, newvalue)
}

func (c *LocalCache) ZRem(key string, value string) (err error) {
	zkey := "Z" + key
	c.setmutex.RLock()
	all, ok := c.sets[zkey]
	c.setmutex.RUnlock()
	if !ok {
		return
	}
	for _, item := range all.Strings() {
		sv := strings.SplitN(item, ":", 2)
		if sv[1] == value {
			c.SRemove(zkey, item)
		}
	}
	return
}

// ----- sort helpers  -------
// this is a helper struct for organizing items according to their priority
type pri struct {
	score int
	val   string
}
type byPri []pri

func (a byPri) Len() int           { return len(a) }
func (a byPri) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byPri) Less(i, j int) bool { return a[i].score < a[j].score }

// ----- end sort helpers  -------

func (c *LocalCache) ZRange(key string, start int, stop int) (values []string, err error) {
	items, err := c.SGet("Z" + key)
	if err != nil {
		return
	}
	priorities := make(byPri, 0, len(items))
	for _, e := range items {
		sv := strings.SplitN(e, ":", 2)
		sc, _ := strconv.Atoi(sv[0])
		priorities = append(priorities, pri{sc, sv[1]})
	}
	sort.Sort(priorities)
	if start < 0 {
		start += len(priorities)
	}
	if stop < 0 {
		stop += len(priorities)
	}

	if start > stop || start >= len(priorities) || stop < 0 {
		values = make([]string, 0)
		return
	}
	if start < 0 {
		start = 0
	}
	if stop > len(priorities) {
		stop = len(priorities) - 1
	}

	p := priorities[start : stop+1]
	for _, item := range p {
		values = append(values, item.val)
	}
	return
}
