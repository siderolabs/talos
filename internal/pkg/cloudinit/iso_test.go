// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cloudinit_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/diskfs/go-diskfs/backend/file"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/cloudinit"
)

func TestISOIsNoCloudAndReproducible(t *testing.T) {
	t.Parallel()

	for _, network := range []string{"", "version: 2\n"} {
		t.Run(network, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			data := cloudinit.Seed{
				MetaData:      "instance-id: a\n",
				UserData:      "#cloud-config\npassword: s3cret\n",
				NetworkConfig: network,
			}
			first := filepath.Join(dir, "first.iso")
			second := filepath.Join(dir, "second.iso")

			require.NoError(t, cloudinit.WriteISO(t.Context(), first, data))
			// The library records filesystem ctime in Rock Ridge TF. Builds in
			// the same second can accidentally appear reproducible.
			time.Sleep(1100 * time.Millisecond)
			require.NoError(t, cloudinit.WriteISO(t.Context(), second, data))

			a, err := os.ReadFile(first)
			require.NoError(t, err)

			b, err := os.ReadFile(second)
			require.NoError(t, err)
			require.True(t, bytes.Equal(a, b), "ISO output changed for identical inputs")
			assertFixedRockRidgeAttributeTimes(t, a)

			info, err := os.Stat(first)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

			image := readISO(t, first)
			require.Equal(t, "CIDATA", strings.TrimRight(image.Label(), "\x00 "))

			entries, err := image.ReadDir(".")
			require.NoError(t, err)

			names := make([]string, 0, len(entries))

			for _, entry := range entries {
				names = append(names, entry.Name())
			}

			wantNames := []string{"meta-data", "user-data"}
			if network != "" {
				wantNames = append(wantNames, "network-config")
			}

			require.ElementsMatch(t, wantNames, names)

			for name, want := range map[string]string{
				"meta-data":      data.MetaData,
				"user-data":      data.UserData,
				"network-config": network,
			} {
				if name == "network-config" && network == "" {
					continue
				}

				require.Equal(t, want, readISOFile(t, image, name))
			}
		})
	}
}

func TestISOEmptyAndCanceled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "seed.iso")
	require.NoError(t, cloudinit.WriteISO(t.Context(), path, cloudinit.Seed{}))

	image := readISO(t, path)

	for _, name := range []string{"meta-data", "user-data"} {
		require.Empty(t, readISOFile(t, image, name))
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	require.ErrorIs(t, cloudinit.WriteISO(ctx, filepath.Join(dir, "canceled.iso"), cloudinit.Seed{}), context.Canceled)
}

func TestISOMaxSeedPreservesPayload(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "max.iso")
	// The payload contains a fake SUSP TF header: it must not be interpreted
	// as directory metadata or modified by timestamp normalization.
	userData := "#cloud-config\n# TF\x1a\x01\x0e" + strings.Repeat("x", cloudinit.MaxSeedBytes-len("#cloud-config\n# TF\x1a\x01\x0e"))
	require.NoError(t, cloudinit.WriteISO(t.Context(), path, cloudinit.Seed{UserData: userData}))
	require.Equal(t, userData, readISOFile(t, readISO(t, path), "user-data"))

	tooLarge := filepath.Join(t.TempDir(), "oversized.iso")
	require.Error(t, cloudinit.WriteISO(t.Context(), tooLarge, cloudinit.Seed{UserData: userData + "x"}))
	_, err := os.Stat(tooLarge)
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Check the serialized metadata, not only the library's interpreted filenames:
// a byte comparison alone could pass if both images retained the same ctime.
func assertFixedRockRidgeAttributeTimes(t *testing.T, image []byte) {
	t.Helper()

	const sector = 2048

	root := image[16*sector+156 : 16*sector+190]
	location := int(binary.LittleEndian.Uint32(root[2:6])) * sector
	length := int(binary.LittleEndian.Uint32(root[10:14]))
	require.LessOrEqual(t, location+length, len(image))

	count := 0

	for pos := location; pos < location+length; {
		record := image[pos : pos+int(image[pos])]
		start := 33 + int(record[32])

		if record[32]%2 == 0 {
			start++
		}

		found := false

		for off := start; off+4 <= len(record); {
			size := int(record[off+2])
			require.GreaterOrEqual(t, size, 4)
			require.LessOrEqual(t, off+size, len(record))

			if string(record[off:off+2]) == "TF" {
				require.False(t, found)
				require.Equal(t, byte(0x0e), record[off+4])
				require.Equal(t, []byte{100, 1, 1, 0, 0, 0, 0}, record[off+19:off+26])

				found = true
			}

			off += size
		}

		require.True(t, found, "missing TF in record %d", count)

		count++
		pos += len(record)
	}

	require.GreaterOrEqual(t, count, 4)
}

func readISO(t *testing.T, path string) *iso9660.FileSystem {
	t.Helper()

	input, err := os.Open(path)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, input.Close())
	})

	info, err := input.Stat()
	require.NoError(t, err)

	image, err := iso9660.Read(file.New(input, true), info.Size(), 0, 0)
	require.NoError(t, err)

	return image
}

func readISOFile(t *testing.T, image *iso9660.FileSystem, name string) string {
	t.Helper()

	entry, err := image.OpenFile("/"+name, os.O_RDONLY)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, entry.Close())
	})

	contents, err := io.ReadAll(entry)
	require.NoError(t, err)

	return string(contents)
}
