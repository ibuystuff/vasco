/**
 * Name: vasco.go
 * Original author: Kent Quirk
 * Created: 12 June 2015
 * Description: Discovery server for The Achievement Network
 * Copyright 2015 The Achievement Network. All rights reserved.
 */

package registry

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/AchievementNetwork/go-util/util"
	"github.com/AchievementNetwork/stringset"
	"github.com/AchievementNetwork/vasco/cache"
)

// Registry maintains a private cache of the registry data
type Registry struct {
	StaticPath       string
	ExpectedServices *stringset.StringSet
	c                cache.Cache
	Timeout          int
}

type StatusItem map[string]interface{}

type StatusBlock []StatusItem

func (s StatusItem) Get(name string) string {
	switch t := s[name].(type) {
	case string:
		return t
	default:
		return ""
	}
}

type byName StatusBlock

func (a byName) Len() int      { return len(a) }
func (a byName) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byName) Less(i, j int) bool {
	return a[i]["Name"].(string)+a[i]["Port"].(string) < a[j]["Name"].(string)+a[j]["Port"].(string)
}

// NewRegistry constructs a registry around a cache, which it accepts as an argument
// (makes it easier to test)
// expected is a space-separated set of strings corresponding to
// server names; status calls will include information about unexpected
// or missing servers.
func NewRegistry(theCache cache.Cache, staticPath string, expected string, timeout int) *Registry {
	r := Registry{
		c:                theCache,
		StaticPath:       staticPath,
		ExpectedServices: stringset.New(),
		Timeout:          timeout,
	}
	exp := strings.Split(expected, " ")
	r.ExpectedServices.Add(exp...)
	// don't allow empty strings in the expected set
	if r.ExpectedServices.Contains("") {
		r.ExpectedServices.Delete("")
	}
	return &r
}

// Register takes a registration object and stores it so that it can be efficiently
// queried. It stores it keyed by its hash value, and if a timeout is requested sets an
// expiration time.
// It also stores its key in a set of items that have been stored, so that it's fast and
// easy to walk a list of all items in the registry.
func (r *Registry) Register(reg *Registration, expire bool) string {
	hash := reg.Hash()

	r.c.Set(hash, reg.String())
	if r.Timeout != 0 && expire {
		// we give clients 2 extra seconds to refresh before timeout
		// in case they're using our timeout to trigger refresh
		r.c.Expire(hash, r.Timeout+2)
	}
	r.c.SAdd("Registry:ITEMS", hash)
	log.Printf("register %s: %v\n", hash, reg.String())
	return hash
}

func (r *Registry) Find(hash string) *Registration {
	regtext, err := r.c.Get(hash)
	if err != nil {
		fmt.Println(err)
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
	r.c.SRemove("Registry:ITEMS", h)
	r.c.Delete(h)
}

func (r *Registry) DetailedStatus() StatusBlock {
	notfound := r.ExpectedServices.Clone()
	statuses := StatusBlock{}
	regs := r.getAllRegistrations()
	for _, reg := range regs {
		u, _ := url.Parse(reg.Address)
		u.Path = reg.Stat.Path
		result, err := http.Get(u.String())
		item := StatusItem{}
		if err != nil {
			item["Error"] = fmt.Sprintf("GET from %s failed.", u.String())
			item["StatusCode"] = http.StatusServiceUnavailable
			if !reg.Disabled {
				reg.Disabled = true
				r.c.Set(reg.Hash(), reg.String())
				// if the service becomes unavailable, expire it in 5 minutes
				r.c.Expire(reg.Hash(), 300)
			}
		} else {
			body, err := ioutil.ReadAll(result.Body)
			err = json.Unmarshal(body, &item)
			if err != nil {
				item["StatusBody"] = string(body)
			}
			item["StatusCode"] = result.StatusCode
			if reg.Disabled {
				reg.Disabled = false
				r.c.Set(reg.Hash(), reg.String())
				r.c.Expire(reg.Hash(), r.Timeout+2)
			}
		}
		item["Name"] = reg.Name
		item["Address"] = reg.Address
		item["Port"] = ""
		item["disabled"] = reg.Disabled
		hs := strings.Split(u.Host, ":")
		if len(hs) == 2 {
			item["Port"] = hs[1]
		}

		if item.Get("Error") == "" {
			item["ID"] = Hash(
				item.Get("starttime"),
				item.Get("Name"),
				item.Get("revision"),
				item.Get("configtype"),
				item.Get("configversion"),
				item.Get("Address"),
				item.Get("Port"),
			)
		}
		if notfound.Contains(item.Get("Name")) {
			notfound.Delete(item.Get("Name"))
		} else {
			// cope with the possibility that we have more than
			// one copy of a service
			if !r.ExpectedServices.Contains(item.Get("Name")) {
				item["unexpected"] = true
			}
		}
		statuses = append(statuses, item)
	}
	for _, name := range notfound.Strings() {
		item := StatusItem{}
		item["Name"] = name
		item["Port"] = ""
		item["Error"] = "Expected service not found."
		item["StatusCode"] = http.StatusServiceUnavailable
		statuses = append(statuses, item)
	}
	sort.Sort(byName(statuses))
	return statuses
}

func (r *Registry) Refresh(reg *Registration) {
	if reg == nil {
		return
	}

	hash := reg.Hash()
	r.c.Expire(hash, r.Timeout+2)
}

// given a set of possible registration options, this chooses one
// of them using a weighted random strategy
func (r *Registry) choose(choices []*Registration) (best *Registration) {
	total := 0
	for _, choice := range choices {
		total += choice.Weight
	}

	target := rand.Intn(total)

	for ix, choice := range choices {
		if target < choice.Weight {
			best = choices[ix]
			return
		} else {
			target -= choice.Weight
		}
	}

	fmt.Printf("WARNING: impossible exit from Registry.choose -- %d %v.\n", target, choices)
	best = choices[len(choices)-1]
	return
}

// getAllRegistrations is a helper function that retrieves all known registrations
// but also removes any that have expired
func (r *Registry) getAllRegistrations() []*Registration {
	hashes, _ := r.c.SGet("Registry:ITEMS")
	results := make([]*Registration, 0)
	removes := make([]string, 0)
	for _, hash := range hashes {
		regtext, err := r.c.Get(hash)
		if err != nil {
			// the hash has expired so plan to delete the corresponding hash item
			removes = append(removes, hash)
		} else {
			reg := NewRegFromJSON(regtext)
			// don't consider disabled registrations
			if !reg.Disabled {
				results = append(results, reg)
			}
		}
	}

	// now delete all the items that expired
	for _, hash := range removes {
		r.c.Delete(hash)
		r.c.SRemove("Registry:ITEMS", hash)
		log.Printf("Expired %s\n", hash)
	}

	return results
}

func (r *Registry) FindBestMatch(surl string) (best *Registration, err error) {
	regs := r.getAllRegistrations()
	matches := make([]*Registration, 0)
	u, _ := url.Parse(surl)
	for _, reg := range regs {
		if reg.regex.MatchString(u.Path) {
			matches = append(matches, reg)
		}
	}

	switch len(matches) {
	case 0:
		log.Printf("No match found for URL '%s'\n", surl)
		best = nil
		err = util.NewWebError(http.StatusNotFound, "VASCO-100", "No matching path was found.")
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

		best = r.choose(choices)
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

	// if we got an error and it's a not found error, then
	// we will forward it to the static server if one is specified
	if err != nil {
		if r.StaticPath == "" {
			fmt.Println("Static path was null so we can't forward the request.")
			return err
		}

		e, ok := err.(*util.WebError)
		if !ok {
			return err
		}

		if e.Code != http.StatusNotFound {
			return err
		}

		reqUrl.Path = r.StaticPath + reqUrl.Path
		target, err = r.FindBestMatch(reqUrl.Path)
		if err != nil {
			fmt.Println("Error - Static lookup failed! ", err.Error())
			return err
		}
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
