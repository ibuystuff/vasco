package cache

import (
	"errors"
	"time"

	"gopkg.in/redis.v3"
)

// an implementation of the Cache that uses redis for the backing store
type RedisCache struct {
	R *redis.Client
}

func NewRedisCache(addr string) *RedisCache {
	c := RedisCache{
		R: redis.NewClient(&redis.Options{Addr: addr}),
	}
	if c.R == nil {
		panic("Unable to open redis client - can't continue.")
	}
	if err := c.R.Set("VASCO_START", "test", 0).Err(); err != nil {
		panic("Failed to set a value in redis:" + err.Error())
	}
	if err := c.R.Get("VASCO_START").Err(); err != nil {
		panic("Failed to get a value from redis:" + err.Error())
	}
	if err := c.R.Del("VASCO_START").Err(); err != nil {
		panic("Failed to delete a value from redis:" + err.Error())
	}

	return &c
}

func (c *RedisCache) Close() {
	c.R.Close()
}

func (c *RedisCache) Set(key string, value string) error {
	return c.R.Set(key, value, 0).Err()
}

func (c *RedisCache) Get(key string) (string, error) {
	return c.R.Get(key).Result()
}

func (c *RedisCache) Delete(key string) error {
	n, err := c.R.Del(key).Result()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("redis: key did not exist")
	}
	return nil
}

func (c *RedisCache) Expire(key string, seconds int) error {
	return c.R.Expire(key, time.Duration(seconds)*time.Second).Err()
}

func (c *RedisCache) ExpireAt(key string, timestamp int64) error {
	return c.R.ExpireAt(key, time.Unix(timestamp, 0)).Err()
}

func (c *RedisCache) SAdd(key string, values ...string) error {
	return c.R.SAdd(key, values...).Err()
}

func (c *RedisCache) SGet(key string) ([]string, error) {
	a, err := c.R.SMembers(key).Result()
	if err != nil {
		return a, err
	}
	if len(a) == 0 {
		return a, errors.New("redis: sget returned empty list")
	}
	return a, nil
}

func (c *RedisCache) SRemove(key string, values ...string) error {
	return c.R.SRem(key, values...).Err()
}

func (c *RedisCache) SCount(key string) (int, error) {
	count, err := c.R.SCard(key).Result()
	if err != nil {
		return int(count), err
	}
	if count == 0 {
		return 0, errors.New("redis: key did not exist to count")
	}
	return int(count), nil
}

func (c *RedisCache) SRandMember(key string) (string, error) {
	return c.R.SRandMember(key).Result()
}

func (c *RedisCache) ZAdd(key string, score int, value string) error {
	zkey := "Z" + key
	return c.R.ZAdd(zkey, redis.Z{Score: float64(score), Member: value}).Err()
}

func (c *RedisCache) ZRem(key string, value string) error {
	zkey := "Z" + key
	return c.R.ZRem(zkey, value).Err()
}

func (c *RedisCache) ZRange(key string, start int, stop int) ([]string, error) {
	zkey := "Z" + key
	return c.R.ZRange(zkey, int64(start), int64(stop)).Result()
}
