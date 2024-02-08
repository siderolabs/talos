// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
)

type mockDocument struct {
	kind, version string
}

func (d mockDocument) Clone() config.Document {
	return d
}

func (d mockDocument) Kind() string {
	return d.kind
}

func (d mockDocument) APIVersion() string {
	return d.version
}

func mockFactory(kind, version string) registry.NewDocumentFunc {
	return func(requestedVersion string) config.Document {
		if requestedVersion == version {
			return mockDocument{kind, version}
		}

		return nil
	}
}

func TestRegistry(t *testing.T) {
	r := registry.NewRegistry()

	// register document types
	r.Register("kind1", mockFactory("kind1", "v1alpha1"))
	r.Register("kind2", mockFactory("kind2", "v1alpha1"))

	// register duplicate kind
	assert.Panics(t, func() {
		r.Register("kind1", mockFactory("kind1", "v1alpha3"))
	})

	// attempt to get unregistered kind
	_, err := r.New("unknownKind", "unknownVersion")
	require.Error(t, err)
	assert.ErrorIs(t, err, registry.ErrNotRegistered)
	assert.EqualError(t, err, "\"unknownKind\" \"unknownVersion\": not registered")

	// successful creation of documents
	d, err := r.New("kind1", "v1alpha1")
	require.NoError(t, err)
	assert.Equal(t, "kind1", d.Kind())
	assert.Equal(t, "v1alpha1", d.APIVersion())

	d, err = r.New("kind2", "v1alpha1")
	require.NoError(t, err)
	assert.Equal(t, "kind2", d.Kind())
	assert.Equal(t, "v1alpha1", d.APIVersion())

	// attempt get registered kind, but wrong version
	_, err = r.New("kind1", "unknownVersion")
	require.Error(t, err)
	assert.ErrorIs(t, err, registry.ErrNotRegistered)
	assert.EqualError(t, err, "\"kind1\" \"unknownVersion\": not registered")
}
