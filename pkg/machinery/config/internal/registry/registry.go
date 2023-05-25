// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package registry provides a registry for configuration documents.
package registry

import (
	"errors"
	"fmt"
	"sync"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

var (
	// ErrNotRegistered indicates that the manifest kind is not registered.
	ErrNotRegistered = errors.New("not registered")
	// ErrExists indicates that the manifest is already registered.
	ErrExists = errors.New("exists")
)

// NewDocumentFunc represents a function that creates a new document by version.
type NewDocumentFunc func(version string) config.Document

var registry = &Registry{
	registered: map[string]NewDocumentFunc{},
}

// Registry represents the document kind/version registry.
type Registry struct {
	m          sync.Mutex
	registered map[string]NewDocumentFunc
}

// Register registers a manifests with the registry.
func Register(kind string, f NewDocumentFunc) {
	registry.register(kind, f)
}

// New creates a new instance of the requested manifest.
func New(kind, version string) (config.Document, error) {
	return registry.new(kind, version)
}

func (r *Registry) register(kind string, f NewDocumentFunc) {
	r.m.Lock()
	defer r.m.Unlock()

	if _, ok := r.registered[kind]; ok {
		panic(ErrExists)
	}

	r.registered[kind] = f
}

func (r *Registry) new(kind, version string) (config.Document, error) {
	r.m.Lock()
	defer r.m.Unlock()

	f, ok := r.registered[kind]
	if ok {
		return f(version), nil
	}

	return nil, fmt.Errorf("%q %q: %w", kind, version, ErrNotRegistered)
}
