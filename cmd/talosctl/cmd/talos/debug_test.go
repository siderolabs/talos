// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows

package talos //nolint:testpackage // to test the unexported debugCmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// TestDebugRejectsTalosContainersNamespace covers talosctl debug refusing --namespace taloscontainers.
//
// The check runs before any client/network setup, so RunE can be called directly and is expected to
// fail on the namespace alone, without needing a reachable cluster.
func TestDebugRejectsTalosContainersNamespace(t *testing.T) {
	oldNamespace := debugCmdFlags.namespace

	t.Cleanup(func() { debugCmdFlags.namespace = oldNamespace })

	debugCmdFlags.namespace = constants.TalosContainersContainerdNamespace

	err := debugCmd.RunE(debugCmd, []string{"docker.io/library/alpine:latest"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), constants.TalosContainersContainerdNamespace)
}

// TestDebugAcceptsOtherNamespaces is the other half: the rejection has to be specific to
// taloscontainers, not a namespace check that would also reject the namespaces debug does support.
//
// Talosconfig is pointed at a path that can't exist so that, once the namespace check passes,
// NewClientFactory fails deterministically on a missing config rather than reaching for a real
// cluster - that failure (not a namespace rejection) is what proves the guard didn't fire.
func TestDebugAcceptsOtherNamespaces(t *testing.T) {
	oldTalosconfig := GlobalArgs.Talosconfig

	t.Cleanup(func() { GlobalArgs.Talosconfig = oldTalosconfig })

	GlobalArgs.Talosconfig = filepath.Join(t.TempDir(), "nonexistent-talosconfig")

	for _, ns := range []string{"inmem", "system", "cri"} {
		t.Run(ns, func(t *testing.T) {
			oldNamespace := debugCmdFlags.namespace

			t.Cleanup(func() { debugCmdFlags.namespace = oldNamespace })

			debugCmdFlags.namespace = ns

			err := debugCmd.RunE(debugCmd, []string{"docker.io/library/alpine:latest"})

			require.Error(t, err)
			assert.NotContains(t, err.Error(), "does not support")
			assert.NotContains(t, err.Error(), constants.TalosContainersContainerdNamespace)
		})
	}
}
