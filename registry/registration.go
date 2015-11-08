/**
 * Name: vasco.go
 * Original author: Kent Quirk
 * Created: 12 June 2015
 * Description: Discovery server for The Achievement Network
 * Copyright 2015 The Achievement Network. All rights reserved.
 */

package registry

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
)

type Status struct {
	Path string `json:"path"`
}

type Registration struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Pattern string `json:"pattern"`
	Weight  int    `json:"weight,omitempty"`
	Stat    Status `json:"status,omitempty"`
	hash    string
	regex   *regexp.Regexp
	url     *url.URL
}

func NewRegFromJSON(j string) *Registration {
	reg := new(Registration)
	dec := json.NewDecoder(strings.NewReader(j))
	if err := dec.Decode(reg); err != nil {
		return nil
	} else {
		reg.SetDefaults()
		return reg
	}
}

func Hash(a ...string) string {
	h := md5.New()
	for _, i := range a {
		io.WriteString(h, i)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (r *Registration) Hash() string {
	// cache the hash
	if r.hash == "" {
		r.hash = Hash(r.Name, r.Address)
	}
	return r.hash
}

func (r *Registration) CompilePath() error {
	pat := "^" + r.Pattern
	if regex, err := regexp.Compile(pat); err != nil {
		return errors.New(fmt.Sprintf("The pattern '%s' is not a valid path expression.", pat))
	} else {
		r.regex = regex
	}
	return nil
}

// processes a registration object and sets defaults for anything not set
func (r *Registration) SetDefaults() error {
	if r.Name == "" {
		return errors.New("The name field cannot be blank.")
	}
	if r.Address == "" {
		return errors.New("The address field cannot be blank.")
	}
	var err error
	if r.url, err = url.Parse(r.Address); err != nil {
		return err
	}
	if r.Pattern == "" {
		return errors.New("The pattern field cannot be blank.")
	}
	if err := r.CompilePath(); err != nil {
		return err
	}
	if r.Stat.Path == "" {
		return errors.New("The status path field cannot be blank.")
	}
	if r.Weight == 0 {
		r.Weight = 100
	}
	return nil
}

func (r *Registration) String() string {
	regtext, _ := json.Marshal(r) // there's no reason this can error, right?
	return string(regtext)
}
