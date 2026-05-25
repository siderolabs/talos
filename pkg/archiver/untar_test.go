// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package archiver_test

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/archiver"
)

func TestUntarCreatesParentDirectories(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)

	payload := []byte("hello")

	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "nested/path/file.txt",
		Mode: 0o644,
		Size: int64(len(payload)),
	}))

	_, err := tw.Write(payload)
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	dir := t.TempDir()

	require.NoError(t, archiver.Untar(context.Background(), bytes.NewReader(buf.Bytes()), dir, nil))

	data, err := os.ReadFile(filepath.Join(dir, "nested/path/file.txt"))
	require.NoError(t, err)
	require.Equal(t, payload, data)
}
