// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs_test

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siderolabs/gen/optional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/makefs"
)

func TestXFSInfo(t *testing.T) { //nolint:tparallel
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")

	for _, test := range []struct {
		name string

		size int64

		expected string
	}{
		{
			name: "1G",

			size: 1024 * 1024 * 1024,

			expected: `meta-data=image isize=512    agcount=4, agsize=65536 blks
         =                       sectsz=512   attr=2, projid32bit=1
         =                       crc=1        finobt=1, sparse=1, rmapbt=1
         =                       reflink=1    bigtime=1 inobtcount=1 nrext64=1
         =                       exchange=1   metadir=0
data     =                       bsize=4096   blocks=262144, imaxpct=25
         =                       sunit=0      swidth=0 blks
naming   =version 2              bsize=4096   ascii-ci=0, ftype=1, parent=1
log      =internal log           bsize=4096   blocks=16384, version=2
         =                       sectsz=512   sunit=0 blks, lazy-count=1
realtime =none                   extsz=4096   blocks=0, rtextents=0
         =                       rgcount=0    rgsize=0 extents
         =                       zoned=0      start=0 reserved=0
`,
		},
		{
			name: "10G",

			size: 10 * 1024 * 1024 * 1024,

			expected: `meta-data=image isize=512    agcount=4, agsize=655360 blks
         =                       sectsz=512   attr=2, projid32bit=1
         =                       crc=1        finobt=1, sparse=1, rmapbt=1
         =                       reflink=1    bigtime=1 inobtcount=1 nrext64=1
         =                       exchange=1   metadir=0
data     =                       bsize=4096   blocks=2621440, imaxpct=25
         =                       sunit=0      swidth=0 blks
naming   =version 2              bsize=4096   ascii-ci=0, ftype=1, parent=1
log      =internal log           bsize=4096   blocks=16384, version=2
         =                       sectsz=512   sunit=0 blks, lazy-count=1
realtime =none                   extsz=4096   blocks=0, rtextents=0
         =                       rgcount=0    rgsize=0 extents
         =                       zoned=0      start=0 reserved=0
`,
		},
		{
			name: "100G",

			size: 100 * 1024 * 1024 * 1024,

			expected: `meta-data=image isize=512    agcount=4, agsize=6553600 blks
         =                       sectsz=512   attr=2, projid32bit=1
         =                       crc=1        finobt=1, sparse=1, rmapbt=1
         =                       reflink=1    bigtime=1 inobtcount=1 nrext64=1
         =                       exchange=1   metadir=0
data     =                       bsize=4096   blocks=26214400, imaxpct=25
         =                       sunit=0      swidth=0 blks
naming   =version 2              bsize=4096   ascii-ci=0, ftype=1, parent=1
log      =internal log           bsize=4096   blocks=16384, version=2
         =                       sectsz=512   sunit=0 blks, lazy-count=1
realtime =none                   extsz=4096   blocks=0, rtextents=0
         =                       rgcount=0    rgsize=0 extents
         =                       zoned=0      start=0 reserved=0
`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()

			tempFile := filepath.Join(tmpDir, "xfs.img")

			require.NoError(t, os.WriteFile(tempFile, nil, 0o644))
			require.NoError(t, os.Truncate(tempFile, test.size))

			require.NoError(t, makefs.XFS(t.Context(), tempFile))

			var stdout bytes.Buffer

			cmd := exec.CommandContext(t.Context(), "xfs_db", "-p", "xfs_info", "-c", "info", tempFile)
			cmd.Stdout = &stdout
			require.NoError(t, cmd.Run())

			actual := strings.ReplaceAll(stdout.String(), tempFile, "image")

			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestXFSCustomSectorSize(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")

	tmpDir := t.TempDir()

	tempFile := filepath.Join(tmpDir, "xfs.img")

	require.NoError(t, os.WriteFile(tempFile, nil, 0o644))
	require.NoError(t, os.Truncate(tempFile, 1024*1024*1024))

	require.NoError(t, makefs.XFS(t.Context(), tempFile, makefs.WithSectorSize(4096)))

	var stdout bytes.Buffer

	cmd := exec.CommandContext(t.Context(), "xfs_db", "-p", "xfs_info", "-c", "info", tempFile)
	cmd.Stdout = &stdout
	require.NoError(t, cmd.Run())

	assert.Contains(t, stdout.String(), "sectsz=4096")
}

func TestXFSReproducibility(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "1732109929")
	t.Setenv("DETERMINISTIC_SEED", "1")
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")

	tmpDir := t.TempDir()

	tempFile := filepath.Join(tmpDir, "reproducible-xfs.img")

	f, err := os.Create(tempFile)
	require.NoError(t, err)

	require.NoError(t, f.Truncate(512*1024*1024))
	require.NoError(t, f.Close())

	sourceDirectory := filepath.Join(tmpDir, "source")

	populateTestDir(t, sourceDirectory, []string{
		"file1.txt",
		"dir1/",
		"dir1/file2.txt",
		"dir1/subdir1/",
		"dir1/subdir1/file3.txt",
	})

	require.NoError(t, makefs.XFS(
		t.Context(),
		tempFile,
		makefs.WithReproducible(true),
		makefs.WithLabel("TESTLABEL"),
		makefs.WithSourceDirectory(sourceDirectory),
	))

	fileData, err := os.Open(tempFile)
	require.NoError(t, err)

	sum1 := sha256.New()

	_, err = io.Copy(sum1, fileData)
	require.NoError(t, err)

	require.NoError(t, fileData.Close())

	// create the filesystem again
	require.NoError(t, makefs.XFS(
		t.Context(),
		tempFile,
		makefs.WithReproducible(true),
		makefs.WithForce(true),
		makefs.WithLabel("TESTLABEL"),
		makefs.WithSourceDirectory(sourceDirectory),
	))

	// get the file sha256 checksum
	fileData, err = os.Open(tempFile)
	require.NoError(t, err)

	sum2 := sha256.New()

	_, err = io.Copy(sum2, fileData)
	require.NoError(t, err)

	require.NoError(t, fileData.Close())

	assert.Equal(t, sum1.Sum(nil), sum2.Sum(nil))
}

func TestXFSConcurrency(t *testing.T) {
	t.Parallel()

	const (
		gib = 1024 * 1024 * 1024
		tib = 1024 * gib
	)

	for _, test := range []struct {
		name string

		deviceSize uint64
		minAGSize  uint64
		numCPU     int

		expected optional.Optional[int]
	}{
		{
			name: "disabled",

			deviceSize: 930 * gib,
			minAGSize:  0,
			numCPU:     128,

			expected: optional.None[int](),
		},
		{
			name: "unknown device size",

			deviceSize: 0,
			minAGSize:  64 * gib,
			numCPU:     128,

			expected: optional.None[int](),
		},
		{
			// smaller than a single allocation group: fall back to the classic geometry, and never
			// emit 1, which mkfs.xfs reads as the magic "number of CPUs" value
			name: "smaller than min AG size",

			deviceSize: 40 * gib,
			minAGSize:  64 * gib,
			numCPU:     128,

			expected: optional.Some(0),
		},
		{
			// exactly one allocation group would fit, still not enough to beat the magic value
			name: "exactly one AG",

			deviceSize: 100 * gib,
			minAGSize:  64 * gib,
			numCPU:     128,

			expected: optional.Some(0),
		},
		{
			name: "capped by min AG size",

			deviceSize: 464 * gib,
			minAGSize:  64 * gib,
			numCPU:     128,

			expected: optional.Some(7),
		},
		{
			name: "capped by min AG size, many cores",

			deviceSize: 1900 * gib,
			minAGSize:  64 * gib,
			numCPU:     384,

			expected: optional.Some(29),
		},
		{
			name: "capped by CPU count",

			deviceSize: 15 * tib,
			minAGSize:  64 * gib,
			numCPU:     32,

			expected: optional.Some(32),
		},
		{
			name: "no CPUs reported",

			deviceSize: 930 * gib,
			minAGSize:  64 * gib,
			numCPU:     0,

			expected: optional.None[int](),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, makefs.XFSConcurrency(test.deviceSize, test.minAGSize, test.numCPU))
		})
	}
}
