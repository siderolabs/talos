// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/config/encoder"
	"github.com/talos-systems/talos/pkg/machinery/config/merge"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

func TestStrategicMergePatch(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir("testdata/strategic")
	require.NoError(t, err)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		t.Run(entry.Name(), testMerge(filepath.Join("testdata/strategic", entry.Name())))
	}
}

func load(t *testing.T, path string) config.Provider {
	provider, err := configloader.NewFromFile(path)
	require.NoError(t, err)

	return provider.Raw().(config.Provider)
}

func testMerge(path string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		left := load(t, filepath.Join(path, "left.yaml"))
		right := load(t, filepath.Join(path, "right.yaml"))
		expected := load(t, filepath.Join(path, "expected.yaml"))

		result := left.(*v1alpha1.Config).DeepCopy()

		err := merge.Merge(result, right)
		require.NoError(t, err)

		marshaled, err := result.EncodeString(encoder.WithComments(encoder.CommentsDisabled))
		require.NoError(t, err)

		assert.Equal(t, expected, result, "got:\n%v", marshaled)
	}
}
