// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	"github.com/siderolabs/talos/internal/pkg/contentlibrary/staging"
)

// fingerprint is the fingerprint of dir, and fails the test rather than returning an error.
func fingerprint(t *testing.T, dir string) string {
	t.Helper()

	got, err := hypervisorctrl.FingerprintLibrary(dir)
	require.NoError(t, err)

	return got
}

// requireFullDigest fails unless s is a whole SHA-256 digest in hex.
func requireFullDigest(t *testing.T, s string) {
	t.Helper()

	require.Len(t, s, hex.EncodedLen(sha256.Size), "the fingerprint must be a whole SHA-256 digest")

	decoded, err := hex.DecodeString(s)
	require.NoError(t, err, "the fingerprint must be hex")
	require.Len(t, decoded, sha256.Size)
}

func TestFingerprintLibraryIsAWholeDigest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// An empty library hashes no entries at all, which pins that nothing else is mixed in.
	empty := fingerprint(t, dir)
	requireFullDigest(t, empty)
	require.Equal(t, hex.EncodeToString(sha256.New().Sum(nil)), empty)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "talos.iso"), []byte("an image"), 0o600))
	requireFullDigest(t, fingerprint(t, dir))
}

func TestFingerprintLibraryTracksContents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	empty := fingerprint(t, dir)
	require.NotEmpty(t, empty)
	require.Equal(t, empty, fingerprint(t, dir), "a fingerprint must not change on its own")

	image := filepath.Join(dir, "talos.iso")
	require.NoError(t, os.WriteFile(image, []byte("an image"), 0o600))

	added := fingerprint(t, dir)
	require.NotEqual(t, empty, added, "an added file must change the fingerprint")
	require.Equal(t, added, fingerprint(t, dir), "rereading an unchanged library must agree")

	require.NoError(t, os.WriteFile(image, []byte("a longer image"), 0o600))
	resized := fingerprint(t, dir)
	require.NotEqual(t, added, resized, "a changed size must change the fingerprint")

	// Same size, different content: only the modification time distinguishes them.
	require.NoError(t, os.WriteFile(image, []byte("a longer imagE"), 0o600))
	require.NoError(t, os.Chtimes(image, time.Time{}, time.Now().Add(time.Hour)))
	require.NotEqual(t, resized, fingerprint(t, dir), "a changed modification time must change the fingerprint")

	require.NoError(t, os.Remove(image))
	require.Equal(t, empty, fingerprint(t, dir), "removing the only file must restore the empty fingerprint")
}

// A rename is what an upload does on its last step, and is the only moment the library's contents
// change: the staged file it renames from is not contents.
func TestFingerprintLibraryIgnoresStagedAndDirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	empty := fingerprint(t, dir)

	staged := filepath.Join(dir, staging.Prefix+"upload"+staging.Suffix)
	require.NoError(t, os.WriteFile(staged, []byte("half an image"), 0o600))
	require.Equal(t, empty, fingerprint(t, dir), "an upload in flight is not contents yet")

	require.NoError(t, os.Mkdir(filepath.Join(dir, "nested"), 0o700))
	require.Equal(t, empty, fingerprint(t, dir), "a library is flat: a directory is not contents")

	require.NoError(t, os.Rename(staged, filepath.Join(dir, "talos.iso")))
	require.NotEqual(t, empty, fingerprint(t, dir), "the rename completing an upload is the change")
}

// The name is part of the fingerprint: two libraries holding the same bytes under different names
// are different contents.
func TestFingerprintLibraryCoversNames(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.iso"), []byte("same"), 0o600))
	withA := fingerprint(t, dir)

	require.NoError(t, os.Rename(filepath.Join(dir, "a.iso"), filepath.Join(dir, "b.iso")))
	require.NotEqual(t, withA, fingerprint(t, dir), "a renamed file must change the fingerprint")
}

func TestFingerprintLibraryReportsAMissingDirectory(t *testing.T) {
	t.Parallel()

	_, err := hypervisorctrl.FingerprintLibrary(filepath.Join(t.TempDir(), "absent"))
	require.ErrorIs(t, err, os.ErrNotExist)
}
