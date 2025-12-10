// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs_test

import (
	"bytes"
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/makefs"
)

func TestXFSInfo(t *testing.T) { //nolint:tparallel
	if hostname, _ := os.Hostname(); hostname != "buildkitsandbox" { //nolint:errcheck
		t.Skipf("skipping test; only run on buildkitsandbox, got %s", hostname)
	}

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
         =                       exchange=0   metadir=0
data     =                       bsize=4096   blocks=262144, imaxpct=25
         =                       sunit=0      swidth=0 blks
naming   =version 2              bsize=4096   ascii-ci=0, ftype=1, parent=0
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
         =                       exchange=0   metadir=0
data     =                       bsize=4096   blocks=2621440, imaxpct=25
         =                       sunit=0      swidth=0 blks
naming   =version 2              bsize=4096   ascii-ci=0, ftype=1, parent=0
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
         =                       exchange=0   metadir=0
data     =                       bsize=4096   blocks=26214400, imaxpct=25
         =                       sunit=0      swidth=0 blks
naming   =version 2              bsize=4096   ascii-ci=0, ftype=1, parent=0
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

			require.NoError(t, makefs.XFS(tempFile))

			var stdout bytes.Buffer

			cmd := exec.CommandContext(t.Context(), "xfs_db", "-p", "xfs_info", "-c", "info", tempFile)
			cmd.Stdout = &stdout
			require.NoError(t, cmd.Run())

			actual := strings.ReplaceAll(stdout.String(), tempFile, "image")

			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestXFSReproducibility(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "1732109929")
	t.Setenv("DETERMINISTIC_SEED", "1")
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")

	tmpDir := t.TempDir()

	tempFile := filepath.Join(tmpDir, "reproducible-xfs.img")

	if _, err := os.Create(tempFile); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	if err := os.Truncate(tempFile, 512*1024*1024); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	if err := makefs.XFS(tempFile,
		makefs.WithReproducible(true),
		makefs.WithLabel("TESTLABEL"),
	); err != nil {
		t.Fatalf("failed to create xfs filesystem: %v", err)
	}

	// get the file sha256 checksum
	fileData, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	sum1 := sha256.Sum256(fileData)

	// create the filesystem again
	if err := makefs.XFS(tempFile,
		makefs.WithReproducible(true),
		makefs.WithForce(true),
		makefs.WithLabel("TESTLABEL"),
	); err != nil {
		t.Fatalf("failed to create xfs filesystem: %v", err)
	}

	// get the file sha256 checksum
	fileData, err = os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	sum2 := sha256.Sum256(fileData)

	assert.Equal(t, sum1, sum2)
}
