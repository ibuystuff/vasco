/**
 * Name: cache.go
 * Original author: Kent Quirk
 * Created: 12 June 2015
 * Description: Cache server. This system is designed to eventually be backed
 *     by Redis, so the data structures emulate those in Redis. But for now it's
 *     just a local memory store for key/value pairs, with expiration.
 * Copyright 2015 The Achievement Network. All rights reserved.
 */

package cache

type Cache interface {
	Close()

	Set(key string, value string) (err error)
	Get(key string) (value string, err error)
	Delete(key string) (err error)
	Expire(key string, seconds int) (err error)
	ExpireAt(key string, timestamp int64) (err error)

	SAdd(key string, values []string) (err error)
	SGet(key string) (values []string, err error)
	SRemove(key string, values []string) (err error)
	SCount(key string) (count int, err error)
	SRandMember(key string) (value string, err error)

	ZAdd(key string, score int, value string) (err error)
	ZRem(key string, value string) (err error)
	ZRange(key string, start int, end int) (values []string, err error)
}
