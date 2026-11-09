// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/qemu"
)

func TestNUMAValidationBeforeSideEffects(t *testing.T) {
	p, err := qemu.NewProvisioner(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, p.Close()) })

	directory := t.TempDir()
	_, err = p.Create(t.Context(), provision.ClusterRequest{
		Name: "invalid-numa", StateDirectory: directory,
		Nodes: provision.NodeRequests{
			{Name: "valid", NanoCPUs: 4_000_000_000, Memory: 1024 * 1024 * 1024},
			{Name: "invalid", NanoCPUs: 1_000_000_000, Memory: 1024 * 1024 * 1024},
		},
	}, provision.WithNUMANodes(2))
	require.ErrorContains(t, err, "node invalid: NUMA node count 2 exceeds vCPU count 1")
	require.NoDirExists(t, filepath.Join(directory, "invalid-numa"))
}

func TestNUMAOption(t *testing.T) {
	opts := provision.DefaultOptions()
	require.Error(t, provision.WithNUMANodes(-1)(&opts))
	require.NoError(t, provision.WithNUMANodes(0)(&opts))
	require.NoError(t, provision.WithNUMANodes(2)(&opts))
	require.Equal(t, 2, opts.NUMANodes)
}
