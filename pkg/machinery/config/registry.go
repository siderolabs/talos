// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"errors"
	"fmt"
	"sync"
)

var (
	// ErrNotRegistered indicates that the manifest kind is not registered.
	ErrNotRegistered = errors.New("not registered")
	// ErrExists indicates that the manifest is already registered.
	ErrExists = errors.New("exists")
)

var registry = &Registry{
	registered: map[string]func(string) interface{}{},
}

// Registry represents the provider registry.
type Registry struct {
	registered map[string]func(string) interface{}

	sync.Mutex
}

// Register registers a manifests with the registry.
func Register(kind string, f func(version string) interface{}) {
	registry.register(kind, f)
}

// New creates a new instance of the requested manifest.
func New(kind, version string) (interface{}, error) {
	return registry.new(kind, version)
}

func (r *Registry) register(kind string, f func(version string) interface{}) {
	r.Lock()
	defer r.Unlock()

	if _, ok := r.registered[kind]; ok {
		panic(ErrExists)
	}

	r.registered[kind] = f
}

func (r *Registry) new(kind, version string) (interface{}, error) {
	r.Lock()
	defer r.Unlock()

	f, ok := r.registered[kind]
	if ok {
		return f(version), nil
	}

	return nil, fmt.Errorf("%q %q: %w", kind, version, ErrNotRegistered)
}
