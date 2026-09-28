// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import "iter"

// LinkResolver resolves link names and aliases to actual link names.
type LinkResolver struct {
	lookup map[string]string // map of link names/aliases to link names
}

// Resolve resolves the link name or alias to the actual link name.
//
// If the link name or alias is not found in the lookup table, it is returned as is.
func (r *LinkResolver) Resolve(name string) string {
	resolved, _ := r.ResolveChecked(name)

	return resolved
}

// ResolveChecked resolves the link name or alias to the actual link name, and reports
// whether the name or alias was found in the lookup table.
//
// If the link name or alias is not found in the lookup table, it is returned as is, with
// found reported as false.
func (r *LinkResolver) ResolveChecked(name string) (resolved string, found bool) {
	if resolved, ok := r.lookup[name]; ok {
		return resolved, true
	}

	return name, false
}

// NewLinkResolver creates a new link name resolver.
func NewLinkResolver(f func() iter.Seq[*LinkStatus]) *LinkResolver {
	lookup := make(map[string]string)

	for link := range f() {
		for alias := range AllLinkAliases(link) {
			lookup[alias] = link.Metadata().ID()
		}
	}

	for link := range f() {
		lookup[link.Metadata().ID()] = link.Metadata().ID()
	}

	return &LinkResolver{lookup: lookup}
}

// NewEmptyLinkResolver creates a new link name resolver with an empty lookup table.
func NewEmptyLinkResolver() *LinkResolver {
	return &LinkResolver{}
}
