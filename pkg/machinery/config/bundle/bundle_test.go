// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bundle_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

func TestGenerateConfig(t *testing.T) {
	configBundleOpts := []bundle.Option{ //nolint:prealloc // this is a test
		bundle.WithInputOptions(
			&bundle.InputOptions{
				ClusterName: "test-cluster",
				Endpoint:    "https://127.0.0.1:6443",
				KubeVersion: "1.222.2",
			},
		),
	}

	patches, err := configpatcher.LoadPatches([]string{"cluster:\n  clusterName: foo\n"})
	require.NoError(t, err)

	configBundleOpts = append(configBundleOpts, bundle.WithPatch(patches))

	configBundle, err := bundle.NewBundle(configBundleOpts...)
	require.NoError(t, err)

	tempDir := t.TempDir()

	require.NoError(t, configBundle.Write(tempDir, encoder.CommentsAll, machine.TypeControlPlane, machine.TypeWorker))

	for _, machineType := range []machine.Type{machine.TypeControlPlane, machine.TypeWorker} {
		var cfg config.Provider

		switch machineType { //nolint:exhaustive
		case machine.TypeControlPlane:
			cfg, err = configloader.NewFromFile(filepath.Join(tempDir, "controlplane.yaml"))
		case machine.TypeWorker:
			cfg, err = configloader.NewFromFile(filepath.Join(tempDir, "worker.yaml"))
		default:
			require.FailNow(t, "unexpected machine type")
		}

		require.NoError(t, err)

		assert.Equal(t, "foo", cfg.Cluster().Name())
	}
}
