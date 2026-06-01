// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package resolver resolves the node names.
package resolver

// Resolver resolves the node names.
type Resolver struct {
	db map[string]string
}

// New creates a new Resolver.
func New(db map[string]string) Resolver {
	return Resolver{
		db: db,
	}
}

// Resolve attempts to resolve the node name.
func (n *Resolver) Resolve(node string) string {
	if resolved, ok := n.db[node]; ok {
		return resolved
	}

	return node
}
