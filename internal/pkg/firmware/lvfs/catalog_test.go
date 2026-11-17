// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvfs_test

import (
	_ "embed"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/firmware/lvfs"
)

//go:embed testdata/metadata.xml
var catalogXML string

const wdcGUID = "3099e006-2dd5-5285-80ec-2845d613dc53"

func TestParseCatalog(t *testing.T) {
	t.Parallel()

	catalog, err := lvfs.ParseCatalog(strings.NewReader(catalogXML), nil)
	require.NoError(t, err)
	assert.NotZero(t, catalog.Len())

	// GUIDs match case-insensitively, each component is returned once
	components := catalog.Lookup([]string{strings.ToUpper(wdcGUID), wdcGUID})
	require.Len(t, components, 1)
	assert.Equal(t, "com.WDC.guid3099e006.firmware", components[0].ID)

	assert.Empty(t, catalog.Lookup([]string{"00000000-0000-0000-0000-000000000000"}))
	assert.Empty(t, catalog.Lookup(nil))
}

func TestParseCatalogKeep(t *testing.T) {
	t.Parallel()

	catalog, err := lvfs.ParseCatalog(strings.NewReader(catalogXML), func(c *lvfs.Component) bool {
		return c.Protocol() == "org.example.none"
	})
	require.NoError(t, err)
	assert.Zero(t, catalog.Len())
	assert.Empty(t, catalog.Lookup([]string{wdcGUID}))
}

func TestParseCatalogInvalid(t *testing.T) {
	t.Parallel()

	_, err := lvfs.ParseCatalog(strings.NewReader("<components><component><id>x</id>"), nil)
	require.Error(t, err)
}
