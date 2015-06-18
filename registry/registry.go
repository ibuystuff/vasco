/**
 * Name: vasco.go
 * Original author: Kent Quirk
 * Created: 12 June 2015
 * Description: Discovery server for The Achievement Network
 * Copyright 2015 The Achievement Network. All rights reserved.
 */

package registry

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"strconv"

	"github.com/AchievementNetwork/vasco/cache"
)

type Registry struct {
	c cache.Cache
}

func NewRegistry(theCache cache.Cache) *Registry {
	return &Registry{c: theCache}
}

func (r *Registry) Register(reg *Registration) {
	stimeout, _ := r.c.Get("Env:DISCOVERY_EXPIRATION")
	timeout, _ := strconv.Atoi(stimeout)

	r.c.Set(reg.Hash(), reg.String())
	if timeout != 0 {
		r.c.Expire(reg.Hash(), timeout)
	}
	r.c.SAdd("Registry:ITEMS", []string{reg.Hash()})
	log.Printf("register %s: %v\n", reg.Hash(), reg.String())
}

func (r *Registry) Find(name, addr string) *Registration {
	hash := Hash(name, addr)
	regtext, err := r.c.Get(hash)
	fmt.Println(regtext)
	if err != nil {
		return nil
	}
	reg := NewRegFromJSON(regtext)
	return reg
}

func (r *Registry) Unregister(reg *Registration) {
	if reg == nil {
		return
	}

	h := reg.Hash()
	r.c.SRemove("Registry:ITEMS", []string{h})
	r.c.Delete(h)
}

func (r *Registry) Refresh(reg *Registration) {
	if reg == nil {
		return
	}

	stimeout, _ := r.c.Get("Env:DISCOVERY_EXPIRATION")
	timeout, _ := strconv.Atoi(stimeout)

	hash := reg.Hash()
	r.c.Expire(hash, timeout)
	log.Printf("Refreshing %s %s\n", reg.Name, reg.Address)
}

func (r *Registry) FindBestMatch(surl string) (best *Registration, err error) {
	hashes, _ := r.c.SGet("Registry:ITEMS")
	matches := make([]*Registration, 0)
	u, _ := url.Parse(surl)
	for _, hash := range hashes {
		regtext, err := r.c.Get(hash)
		if err != nil {
			// the hash has expired so delete the corresponding hash item
			r.c.Delete(hash)
			r.c.SRemove("Registry:ITEMS", []string{hash})
			log.Printf("Expired %s\n", hash)
			// and call ourselves recursively
			return r.FindBestMatch(surl)
		} else {
			reg := NewRegFromJSON(regtext)
			if reg.regex.MatchString(u.Path) {
				matches = append(matches, reg)
			}
		}
	}

	switch len(matches) {
	case 0:
		log.Printf("No match found for URL '%s'\n", surl)
		best = nil
		err = errors.New("No matching path was found.") // should be 404
	case 1:
		err = nil
		best = matches[0]
	default:
		// at least two patterns were matched, so now we need to compare them for
		// matching length. If we had these two patterns:
		//   /foo(/.*)
		//   /foo/bar(/.*)
		// and we get /foo/bar/bazz, it will match both, but we want to return
		// the second -- so we calculate the length of the unparenthesized portion
		// of our match
		var choices []*Registration
		bestlen := 0
		for _, match := range matches {
			subs := match.regex.FindStringSubmatch(u.Path)
			matchedlen := len(subs[0])
			if len(subs) > 1 {
				matchedlen -= len(subs[1])
			}
			if matchedlen > bestlen {
				bestlen = matchedlen
				choices = []*Registration{match}
			} else if matchedlen == bestlen {
				choices = append(choices, match)
			}
		}

		// this is the random strategy
		best = choices[rand.Intn(len(choices))]
	}

	if best != nil {
		log.Printf("Selected '%s' on '%s' for URL '%s'\n", best.Name, best.Address, surl)
	}
	return
}

// Requirement:
// Given a request, match it with the set of paths and rewrite it to forward it

func (r *Registry) RewriteUrl(reqUrl *url.URL) error {
	target, err := r.FindBestMatch(reqUrl.Path)
	if err != nil {
		return err
	}

	// if the registration pattern included parentheses, we're going to
	// rewrite the URL path
	matches := target.regex.FindStringSubmatch(reqUrl.Path)
	if len(matches) > 1 {
		reqUrl.Path = matches[1]
	}

	reqUrl.Scheme = target.url.Scheme
	reqUrl.Host = target.url.Host
	return nil
}
