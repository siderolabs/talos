// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	tests := map[string]struct {
		original string
		golden   string
	}{
		"default_test": {
			original: "a.orig",
			golden:   "a.golden",
		},
	}

	for name, test := range tests {
		test := test

		t.Run(name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "go-pkg-*")
			require.NoError(t, err)

			t.Cleanup(func() {
				require.NoError(t, os.RemoveAll(tempDir))
			})

			tmpFile := filepath.Join(tempDir, "my.go")
			origPath := filepath.Join("testdata", test.original)

			require.NoError(t, CopyFile(origPath, tmpFile))

			err = Run(tempDir)
			require.NoError(t, err)

			fileData := string(must(os.ReadFile(tmpFile))(t))
			goldenPath := filepath.Join("testdata", test.golden)
			goldenData := string(must(os.ReadFile(goldenPath))(t))

			require.Equal(t, goldenData, fileData)
		})
	}
}

func must[V any](v V, err error) func(t *testing.T) V {
	return func(t *testing.T) V {
		require.NoError(t, err)

		return v
	}
}
