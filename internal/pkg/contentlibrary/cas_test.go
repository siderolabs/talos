// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package contentlibrary_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/contentlibrary"
)

func TestRemoveObjectUnreferenced(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	root, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })

	hash := digest.SHA256.FromString("payload").String()

	require.NoError(t, os.WriteFile(filepath.Join(path, "stage"), []byte("payload"), 0o600))

	object, err := contentlibrary.PublishObject(root, "stage", hash)
	require.NoError(t, err)
	require.NoError(t, contentlibrary.RemoveObject(root, hash, false))
	require.NoFileExists(t, filepath.Join(path, object))
	require.ErrorIs(t, contentlibrary.RemoveObject(root, hash, false), os.ErrNotExist)
	_, err = root.Lstat("stage")
	require.NoError(t, err)
}

func TestRemoveObjectRefusesReferencesWithoutMutation(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	root, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })

	hash := digest.SHA256.FromString("payload").String()

	require.NoError(t, os.WriteFile(filepath.Join(path, "stage"), []byte("payload"), 0o600))
	require.NoError(t, contentlibrary.Publish(root, "stage", "z.iso", hash))
	object, err := contentlibrary.ObjectPath(hash)
	require.NoError(t, err)
	require.NoError(t, root.Symlink(object, "a.iso"))
	require.NoError(t, root.Mkdir(".cloud-init-owners", 0o700))
	require.NoError(t, root.Link("a.iso", ".cloud-init-owners/witness"))
	err = contentlibrary.RemoveObject(root, hash, false)
	require.ErrorIs(t, err, contentlibrary.ErrObjectReferenced)
	require.ErrorContains(t, err, "a.iso")
	require.ErrorContains(t, err, "z.iso")
	require.ErrorContains(t, err, ".cloud-init-owners/witness")
	require.ErrorContains(t, err, "force")
	require.FileExists(t, filepath.Join(path, object))
	require.FileExists(t, filepath.Join(path, "a.iso"))
	require.FileExists(t, filepath.Join(path, "z.iso"))
	require.True(t, errors.Is(err, contentlibrary.ErrObjectReferenced))
}

func TestRemoveObjectRejectsAmbiguousReferenceEvenWithForce(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	root, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })

	hash := digest.SHA256.FromString("payload").String()

	require.NoError(t, os.WriteFile(filepath.Join(path, "stage"), []byte("payload"), 0o600))

	object, err := contentlibrary.PublishObject(root, "stage", hash)
	require.NoError(t, err)
	require.NoError(t, root.Symlink("../outside/"+filepath.Base(object), "suspicious.iso"))
	require.ErrorContains(t, contentlibrary.RemoveObject(root, hash, true), "unclassifiable")
	_, err = root.Lstat("suspicious.iso")
	require.NoError(t, err)
	_, err = root.Lstat(object)
	require.NoError(t, err)
}

func TestRemoveObjectForceRemovesOnlyReferencingLinks(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	root, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })

	hash := digest.SHA256.FromString("payload").String()

	require.NoError(t, os.WriteFile(filepath.Join(path, "stage"), []byte("payload"), 0o600))
	require.NoError(t, contentlibrary.Publish(root, "stage", "image.iso", hash))
	object, err := contentlibrary.ObjectPath(hash)
	require.NoError(t, err)
	require.NoError(t, root.Symlink("image.iso", "chain.iso"))
	require.NoError(t, root.Mkdir(".cloud-init-owners", 0o700))
	require.NoError(t, root.Link("image.iso", ".cloud-init-owners/witness"))
	require.NoError(t, root.Symlink("stage", "unrelated.iso"))
	require.NoError(t, root.Symlink("unrelated.iso", "unrelated-chain.iso"))
	require.NoError(t, contentlibrary.RemoveObject(root, hash, true))

	for _, name := range []string{object, "image.iso", "chain.iso", ".cloud-init-owners/witness"} {
		_, err = root.Lstat(name)
		require.ErrorIs(t, err, os.ErrNotExist, name)
	}

	for _, name := range []string{"stage", "unrelated.iso", "unrelated-chain.iso"} {
		_, err = root.Lstat(name)
		require.NoError(t, err, name)
	}
}

func TestRemoveObjectRejectsSymlinkObjectDirectory(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	root, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })

	hash := digest.SHA256.FromString("payload").String()
	object, err := contentlibrary.ObjectPath(hash)
	require.NoError(t, err)
	require.NoError(t, root.Mkdir("other", 0o700))
	require.NoError(t, root.Mkdir("objects", 0o700))
	require.NoError(t, root.Symlink("../other", "objects/sha256"))
	require.NoError(t, os.WriteFile(filepath.Join(path, "other", filepath.Base(object)), []byte("payload"), 0o600))
	require.Error(t, contentlibrary.RemoveObject(root, hash, true))
	require.FileExists(t, filepath.Join(path, "other", filepath.Base(object)))
}

func TestRemoveObjectRejectsInvalidOrNonregular(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	root, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	require.Error(t, contentlibrary.RemoveObject(root, "sha256:../../escape", true))

	hash := digest.SHA256.FromString("payload").String()
	object, err := contentlibrary.ObjectPath(hash)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(path, filepath.Dir(object)), 0o700))
	require.NoError(t, root.Symlink("../../outside", object))
	require.ErrorContains(t, contentlibrary.RemoveObject(root, hash, true), "not a regular file")
	_, err = root.Lstat(object)
	require.NoError(t, err)
}

func TestRemoveAliasPreservesObject(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	root, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })

	hash := digest.SHA256.FromString("payload").String()

	require.NoError(t, os.WriteFile(filepath.Join(path, "stage"), []byte("payload"), 0o600))
	require.NoError(t, contentlibrary.Publish(root, "stage", "image.iso", hash))
	require.Error(t, contentlibrary.RemoveAlias(root, "../image.iso"))
	require.NoError(t, contentlibrary.RemoveAlias(root, "image.iso"))
	require.ErrorIs(t, contentlibrary.RemoveAlias(root, "image.iso"), os.ErrNotExist)

	object, err := contentlibrary.ObjectPath(hash)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(path, object))
}

func TestPublishWaitsForMutationAcrossRootHandles(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	first, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, first.Close()) })

	second, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, second.Close()) })
	require.NoError(t, os.WriteFile(filepath.Join(path, "stage"), []byte("payload"), 0o600))

	hash := digest.SHA256.FromString("payload").String()
	entered := make(chan struct{})
	release := make(chan struct{})

	guardDone := make(chan error, 1)
	go func() {
		guardDone <- contentlibrary.WithMutation(first, func() error {
			close(entered)
			<-release

			return nil
		})
	}()

	<-entered

	publishDone := make(chan error, 1)
	go func() { publishDone <- contentlibrary.Publish(second, "stage", "image.iso", hash) }()

	select {
	case err := <-publishDone:
		close(release)
		<-guardDone
		t.Fatalf("publication crossed held mutation guard: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-guardDone)
	require.NoError(t, <-publishDone)

	_, err = contentlibrary.Resolve(first, "image.iso")
	require.NoError(t, err)
}

func TestPublishDeduplicatesObjects(t *testing.T) {
	t.Parallel()
	path := t.TempDir()
	root, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })

	hash := digest.SHA256.FromString("payload").String()

	for _, name := range []string{"first.iso", "second.iso"} {
		stage := "." + name + ".upload"
		require.NoError(t, os.WriteFile(filepath.Join(path, stage), []byte("payload"), 0o600))
		require.NoError(t, contentlibrary.Publish(root, stage, name, hash))
	}

	first, err := root.Readlink("first.iso")
	require.NoError(t, err)
	second, err := root.Readlink("second.iso")
	require.NoError(t, err)
	require.Equal(t, first, second)

	objects, err := os.ReadDir(filepath.Join(path, "objects", "sha256"))
	require.NoError(t, err)
	require.Len(t, objects, 1)
}

func TestPublishAndResolve(t *testing.T) {
	t.Parallel()
	path := t.TempDir()
	root, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	require.NoError(t, os.WriteFile(filepath.Join(path, ".stage.upload"), []byte("payload"), 0o600))

	hash := digest.SHA256.FromString("payload").String()
	require.NoError(t, contentlibrary.Publish(root, ".stage.upload", "name.iso", hash))
	resolved, err := contentlibrary.Resolve(root, "name.iso")
	require.NoError(t, err)
	require.Equal(t, filepath.Join("objects", "sha256", digest.SHA256.FromString("payload").Encoded()), resolved)
	require.ErrorIs(t, contentlibrary.Publish(root, ".stage.upload", "name.iso", hash), os.ErrExist)
	require.NoError(t, os.Symlink("../../escape", filepath.Join(path, "escape.iso")))

	_, err = contentlibrary.Resolve(root, "escape.iso")
	require.Error(t, err)
}

func TestResolveRejectsSymlinkObject(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	root, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, root.Close())
	})

	object, err := contentlibrary.ObjectPath(digest.SHA256.FromString("image").String())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(path, filepath.Dir(object)), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(path, "mutable"), []byte("image"), 0o600))
	require.NoError(t, root.Symlink("../../mutable", object))
	require.NoError(t, root.Symlink(object, "image.raw"))

	_, err = contentlibrary.Resolve(root, "image.raw")
	require.Error(t, err, "only names may be symlinks; object paths must contain regular files")
}

func TestPublishObjectRejectsSymlinkObject(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	root, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, root.Close())
	})

	hash := digest.SHA256.FromString("image").String()
	object, err := contentlibrary.ObjectPath(hash)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(path, filepath.Dir(object)), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(path, "mutable"), []byte("image"), 0o600))
	require.NoError(t, root.Symlink("../../mutable", object))

	_, err = contentlibrary.PublishObject(root, "mutable", hash)
	require.Error(t, err, "deduplication must not adopt a symlink as an immutable object")
}

func TestPublishRejectsIncorrectIdentityBeforeInstallingObject(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	root, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })

	hash := digest.SHA256.FromString("expected").String()
	object, err := contentlibrary.ObjectPath(hash)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(path, "staged"), []byte("different"), 0o600))

	require.Error(t, contentlibrary.Publish(root, "staged", "image.raw", hash))
	require.NoFileExists(t, filepath.Join(path, object))
	require.NoFileExists(t, filepath.Join(path, "image.raw"))
}
