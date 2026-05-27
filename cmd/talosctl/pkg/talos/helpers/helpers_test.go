// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
)

type cfg struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
}

func TestExtractFileFromTarGz(t *testing.T) {
	file, err := os.Open("./testdata/archive.tar.gz")
	assert.NoError(t, err)

	data, err := helpers.ExtractFileFromTarGz("kubeconfig", file)
	assert.NoError(t, err)

	// just some primitive sanity check that yaml file inside was not corrupted somehow
	var c cfg

	err = yaml.Unmarshal(data, &c)
	assert.NoError(t, err)

	assert.Equal(t, c.APIVersion, "v1")
	assert.Equal(t, c.Kind, "Config")

	_, err = helpers.ExtractFileFromTarGz("void", file)
	assert.Error(t, err)
}

func TestExtractTarGzCreatesParentDirectories(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	payload := []byte("hello")

	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "nested/path/file.txt",
		Mode: 0o644,
		Size: int64(len(payload)),
	}))

	_, err := tw.Write(payload)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gzw.Close())

	dir := t.TempDir()

	require.NoError(t, helpers.ExtractTarGz(dir, io.NopCloser(bytes.NewReader(buf.Bytes()))))

	data, err := os.ReadFile(filepath.Join(dir, "nested/path/file.txt"))
	require.NoError(t, err)
	require.Equal(t, payload, data)
}
