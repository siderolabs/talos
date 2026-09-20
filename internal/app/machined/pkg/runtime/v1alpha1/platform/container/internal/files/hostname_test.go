// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/container/internal/files"
)

func TestReadHostname(t *testing.T) {
	t.Parallel()

	spec, err := files.ReadHostname("testdata/hostname")
	require.NoError(t, err)

	require.Equal(t, "foo", spec.Hostname)
	require.Equal(t, "example.com", spec.Domainname)
}

// An empty /etc/hostname is a hostname the container runtime did not set, not a
// malformed one: it must not fail the platform network configuration, which would
// also drop the resolvers and the platform metadata read alongside it.
func TestReadHostnameEmpty(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		contents string
	}{
		{name: "empty", contents: ""},
		{name: "newline", contents: "\n"},
		{name: "whitespace", contents: " \t\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "hostname")
			require.NoError(t, os.WriteFile(path, []byte(test.contents), 0o644))

			spec, err := files.ReadHostname(path)
			require.NoError(t, err)
			require.Empty(t, spec.Hostname)
		})
	}
}

func TestReadHostnameMissing(t *testing.T) {
	t.Parallel()

	spec, err := files.ReadHostname(filepath.Join(t.TempDir(), "nonexistent"))
	require.NoError(t, err)
	require.Empty(t, spec.Hostname)
}
