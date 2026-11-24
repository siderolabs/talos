// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sysfs_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/sysfs"
)

// countReader returns a fixed byte count from Read, as the kernel does for a sysfs show()
// callback which returns a negative value that is not an errno.
type countReader int

func (r countReader) Read([]byte) (int, error) {
	return int(r), nil
}

func TestReadAll(t *testing.T) {
	t.Parallel()

	large := bytes.Repeat([]byte("0123456789"), 1000)

	for _, test := range []struct {
		name string
		r    io.Reader
		want []byte
	}{
		{name: "empty", r: bytes.NewReader(nil), want: []byte{}},
		{name: "attribute", r: bytes.NewReader([]byte("2400000\n")), want: []byte("2400000\n")},
		{name: "large", r: bytes.NewReader(large), want: large},
		{name: "one byte at a time", r: iotest.OneByteReader(bytes.NewReader(large)), want: large},
		{name: "data with EOF", r: iotest.DataErrReader(bytes.NewReader([]byte("performance\n"))), want: []byte("performance\n")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := sysfs.ReadAll(test.r)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestReadAllInvalidCount(t *testing.T) {
	t.Parallel()

	for _, n := range []int{-1892423448, -1, 1 << 20} {
		_, err := sysfs.ReadAll(countReader(n))
		require.ErrorIs(t, err, sysfs.ErrInvalidReadCount)
	}
}

func TestReadAllError(t *testing.T) {
	t.Parallel()

	errTest := errors.New("test")

	_, err := sysfs.ReadAll(iotest.ErrReader(errTest))
	require.ErrorIs(t, err, errTest)
}

func TestReadFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "scaling_governor"), []byte("schedutil\n"), 0o644))

	got, err := sysfs.ReadFile(filepath.Join(dir, "scaling_governor"))
	require.NoError(t, err)
	assert.Equal(t, []byte("schedutil\n"), got)

	_, err = sysfs.ReadFile(filepath.Join(dir, "missing"))
	require.ErrorIs(t, err, fs.ErrNotExist)
}
