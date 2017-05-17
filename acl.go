package main

import (
	"encoding/json"
	"io"
	"regexp"

	"github.com/davecgh/go-spew/spew"
)

// accessController interface for access control.
type accessController interface {
	loadRules(r io.Reader) error
	skip(path string) bool
}

// newPathAccessController returns a new access control list to determine the paths
// that should not be checked for authentication. An io.Reader
// is passed in for us to configure the rules needed to evaluate
// the path of each request.
func newPathAccessController(r io.Reader) (*pathAccessController, error) {
	pac := pathAccessController{}
	if err := pac.loadRules(r); err != nil {
		return nil, err
	}

	return &pac, nil
}

var _ accessController = (*pathAccessController)(nil)

// pathAccessController tracks the request paths that should not be checked for authentication.
type pathAccessController struct {
	rules []Rule
}

// Rule encapsulates the path regex to match request paths against
// when determining if a rule applies to a request.
type Rule struct {
	pathRegex *regexp.Regexp
	Path      string `json:"path"`
	Skip      bool   `json:"skip"`
}

// loadRules loads the access control rules from the reader.
func (pac *pathAccessController) loadRules(r io.Reader) error {
	var rules []Rule
	err := json.NewDecoder(r).Decode(&rules)
	if err != nil {
		return err
	}

	for _, ru := range rules {
		if ru.pathRegex, err = regexp.Compile(ru.Path); err != nil {
			return err
		}
		pac.rules = append(pac.rules, ru)
	}

	return nil
}

// skip finds a match for the given path and if found
// returns the bool associated with that rule.
func (pac *pathAccessController) skip(path string) bool {
	for _, r := range pac.rules {
		if r.pathRegex.MatchString(path) {
			spew.Printf("found match for %s, using regex %s, skipping\n", path, r.Path)
			return r.Skip
		}
	}
	return false
}
