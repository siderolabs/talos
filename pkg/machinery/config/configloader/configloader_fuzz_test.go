// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configloader_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzConfigLoader(f *testing.F) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.test"))
	require.NoError(f, err)

	for _, file := range files {
		b, err := os.ReadFile(file)
		require.NoError(f, err)
		f.Add(b)
	}

	f.Add([]byte(":   \xea"))
	f.Add([]byte(nil))
	f.Add([]byte(""))

	f.Fuzz(func(t *testing.T, b []byte) {
		t.Parallel()

		testConfigLoaderBytes(t, b, false)
	})
}
