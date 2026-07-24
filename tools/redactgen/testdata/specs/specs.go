// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package specs is the input for the redactgen tests.
package specs

import "net/url"

// SecretKey is a named string type.
type SecretKey string

// ConfigSpec exercises all supported ways to reach a sensitive field.
//
//redactgen:gen
type ConfigSpec struct {
	Name     string
	Count    int
	Token    string    `redact:"replace"`
	Key      SecretKey `redact:"replace"`
	Raw      []byte    `redact:"zero"`
	Endpoint *url.URL  `redact:"replace"`
	Location url.URL   `redact:"replace"`

	Nested   NestedSpec
	Optional *NestedSpec
	Plain    PlainSpec

	Items    []ItemSpec
	Pointers []*ItemSpec
	ByName   map[string]ItemSpec
	ByID     map[string]*ItemSpec
}

// DeepCopy is a stub: redactgen only checks that the method is there.
func (spec ConfigSpec) DeepCopy() ConfigSpec {
	return spec
}

// PlainSpec carries no sensitive data at all.
//
//redactgen:gen
type PlainSpec struct {
	Name string
	Path string
}

// NestedSpec is reached both by value and by pointer.
type NestedSpec struct {
	Password string `redact:"replace"`
	Comment  string
}

// ItemSpec is reached through slices and maps.
type ItemSpec struct {
	Name   string
	Secret string `redact:"replace"`
}
