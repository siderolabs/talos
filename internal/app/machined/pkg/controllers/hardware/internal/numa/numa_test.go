// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package numa_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hardware/internal/numa"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

func TestValidateMembership(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cpuPath := filepath.Join(root, "devices/system/cpu")

	require.NoError(t, os.MkdirAll(cpuPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cpuPath, "present"), []byte("0-1\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(cpuPath, "online"), []byte("0-1\n"), 0o644))

	current := &hardware.NUMATopologySpec{PresentCPUs: []uint32{0, 1}, OnlineCPUs: []uint32{0, 1}}
	require.NoError(t, numa.ValidateMembership(root, current, nil, true))

	// CPU 1 came online after this snapshot.
	stale := &hardware.NUMATopologySpec{PresentCPUs: []uint32{0, 1}, OnlineCPUs: []uint32{0}}
	require.ErrorIs(t, numa.ValidateMembership(root, stale, nil, true), numa.ErrTopologyChanged)
}
