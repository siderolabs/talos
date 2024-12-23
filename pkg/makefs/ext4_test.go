// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makefs_test

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/makefs"
)

// TestExt4Reproducibility tests that the ext4 filesystem is reproducible.
func TestExt4Reproducibility(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "1732109929")
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")

	tmpDir := t.TempDir()

	tempFile := filepath.Join(tmpDir, "reproducible-ext4.img")

	if _, err := os.Create(tempFile); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	if err := os.Truncate(tempFile, 512*1024*1024); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	if err := makefs.Ext4(tempFile, makefs.WithReproducible(true)); err != nil {
		t.Fatalf("failed to create ext4 filesystem: %v", err)
	}

	// get the file sha256 checksum
	fileData, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	sum1 := sha256.Sum256(fileData)

	// create the filesystem again
	if err := makefs.Ext4(tempFile, makefs.WithReproducible(true), makefs.WithForce(true)); err != nil {
		t.Fatalf("failed to create ext4 filesystem: %v", err)
	}

	// get the file sha256 checksum
	fileData, err = os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	sum2 := sha256.Sum256(fileData)

	assert.Equal(t, sum1, sum2)
}

// TestExt4Resize tests that the ext4 filesystem can be resized.
func TestExt4Resize(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")

	tmpDir := t.TempDir()

	tempFile := filepath.Join(tmpDir, "resize-ext4.img")

	if _, err := os.Create(tempFile); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	if err := os.Truncate(tempFile, 64*1024*1024); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	if err := makefs.Ext4(tempFile); err != nil {
		t.Fatalf("failed to create ext4 filesystem: %v", err)
	}

	if err := os.Truncate(tempFile, 128*1024*1024); err != nil {
		t.Fatalf("failed to resize file: %v", err)
	}

	assert.NoError(t, makefs.Ext4Resize(tempFile))
}
