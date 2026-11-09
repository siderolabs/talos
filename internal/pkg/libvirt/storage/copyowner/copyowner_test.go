// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package copyowner_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/libvirt/storage/copyowner"
	"github.com/siderolabs/talos/internal/pkg/qemuimg"
)

func TestCopyBindingRequiresMatchingOwnershipAndSurvivesRestart(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(target, ".copy-on-use"), 0o700))

	key := fmt.Sprintf("%x", sha256.Sum256([]byte("intent")))
	name := "vm__install__digest.raw"
	require.NoError(t, os.WriteFile(filepath.Join(target, name), []byte("image"), 0o600))
	require.Error(t, new(copyowner.Bindings).Record(target, key, name))
	witness := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
	require.NoError(t, os.Link(filepath.Join(target, name), filepath.Join(target, ".copy-on-use", witness)))
	require.NoError(t, new(copyowner.Bindings).Record(target, key, name))
	got, err := copyowner.ReadBinding(target, key)
	require.NoError(t, err)
	require.Equal(t, name, got)
	require.NoError(t, os.Remove(filepath.Join(target, name)))
	got, err = copyowner.ReadBinding(target, key)
	require.Error(t, err)
	require.Empty(t, got)
}

func TestCopyBindingDoesNotGuessBetweenCompletedCopies(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(target, ".copy-on-use"), 0o700))

	key := fmt.Sprintf("%x", sha256.Sum256([]byte("intent")))

	for _, name := range []string{"a.raw", "b.raw"} {
		require.NoError(t, os.WriteFile(filepath.Join(target, name), []byte(name), 0o600))
		witness := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
		require.NoError(t, os.Link(filepath.Join(target, name), filepath.Join(target, ".copy-on-use", witness)))
	}

	got, err := copyowner.ReadBinding(target, key)
	require.NoError(t, err)
	require.Empty(t, got)
	require.NoError(t, new(copyowner.Bindings).Record(target, key, "a.raw"))
	got, err = copyowner.ReadBinding(target, key)
	require.NoError(t, err)
	require.Equal(t, "a.raw", got)
	require.NoError(t, new(copyowner.Bindings).Record(target, key, "b.raw"))
	got, err = copyowner.ReadBinding(target, key)
	require.NoError(t, err)
	require.Equal(t, "b.raw", got)
}

func TestCopyBindingWarmRecordStable(t *testing.T) {
	target := t.TempDir()
	key := strings.Repeat("a", 64)
	name := "a.raw"

	require.NoError(t, os.Mkdir(filepath.Join(target, ".copy-on-use"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(target, name), []byte("image"), 0o600))
	witness := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
	require.NoError(t, os.Link(filepath.Join(target, name), filepath.Join(target, ".copy-on-use", witness)))

	bindings := copyowner.Bindings{}
	require.NoError(t, bindings.Record(target, key, name))
	path := filepath.Join(target, ".copy-on-use", "binding-"+key)
	before, err := os.Stat(path)
	require.NoError(t, err)

	for range 3 {
		require.NoError(t, bindings.Record(target, key, name))
	}

	after, err := os.Stat(path)
	require.NoError(t, err)
	require.True(t, os.SameFile(before, after), "warm observations must not rewrite bindings")

	cold := copyowner.Bindings{}
	require.NoError(t, cold.Record(target, key, name))
}

func TestCopyBindingRejectsMalformedRecord(t *testing.T) {
	for _, test := range []struct {
		name     string
		contents string
		symlink  bool
	}{
		{name: "empty"},
		{name: "oversize", contents: strings.Repeat("a", 256)},
		{name: "traversal", contents: "../a.raw"},
		{name: "symlink", contents: "a.raw", symlink: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := t.TempDir()
			key := strings.Repeat("b", 64)

			require.NoError(t, os.Mkdir(filepath.Join(target, ".copy-on-use"), 0o700))

			record := filepath.Join(target, ".copy-on-use", "binding-"+key)
			if test.symlink {
				require.NoError(t, os.WriteFile(filepath.Join(target, "record"), []byte(test.contents), 0o600))
				require.NoError(t, os.Symlink("../record", record))
			} else {
				require.NoError(t, os.WriteFile(record, []byte(test.contents), 0o600))
			}

			_, err := copyowner.ReadBinding(target, key)
			require.Error(t, err)
		})
	}
}

func TestCopyPublicationAbsentRejectsWitnessOnly(t *testing.T) {
	target := t.TempDir()
	name := "a.raw"

	require.NoError(t, os.Mkdir(filepath.Join(target, ".copy-on-use"), 0o700))

	witness := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".copy-on-use", witness), []byte("image"), 0o600))
	absent, err := copyowner.PublicationAbsent(target, name)
	require.NoError(t, err)
	require.False(t, absent, "a partial publication is not proof that rebinding is safe")
}

func TestCopyPublicationAbsentRejectsMalformedWritableMarker(t *testing.T) {
	for _, content := range []string{"foreign", "image-owner:unknown\n", "image-owner:raw:not-a-uuid\n", "image-inode:1:2:raw:00000000-0000-0000-0000-000000000000\n"} {
		t.Run(content, func(t *testing.T) {
			target := t.TempDir()
			private := filepath.Join(target, ".copy-on-use")
			require.NoError(t, os.Mkdir(private, 0o700))

			marker := fmt.Sprintf("image-%x", sha256.Sum256([]byte("guest.img")))
			require.NoError(t, os.WriteFile(filepath.Join(private, marker), []byte(content), 0o600))

			absent, err := copyowner.PublicationAbsent(target, "guest.img")
			require.False(t, absent)
			require.Error(t, err)
		})
	}
}

func TestCopyBindingRejectsInvalidFirstWitness(t *testing.T) {
	for _, invalid := range []string{"foreign", "symlink", "directory"} {
		t.Run(invalid, func(t *testing.T) {
			target := t.TempDir()
			private := filepath.Join(target, ".copy-on-use")
			require.NoError(t, os.Mkdir(private, 0o700))

			name := "guest.img"
			path := filepath.Join(target, name)
			require.NoError(t, os.WriteFile(path, []byte("guest data"), 0o600))

			witness := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))

			owner := imageOwnerFixtureToken(t, path)
			marker := "image-owner:raw:" + owner + "\n"
			require.NoError(t, os.WriteFile(filepath.Join(private, "image-"+witness), []byte(marker), 0o600))

			key := fmt.Sprintf("%x", sha256.Sum256([]byte("intent")))
			require.NoError(t, new(copyowner.Bindings).Record(target, key, name))

			first := filepath.Join(private, witness)

			switch invalid {
			case "foreign":
				require.NoError(t, os.WriteFile(first, []byte("foreign"), 0o600))
			case "symlink":
				require.NoError(t, os.Symlink(path, first))
			case "directory":
				require.NoError(t, os.Mkdir(first, 0o700))
			}

			_, err := copyowner.ReadBinding(target, key)
			require.ErrorContains(t, err, "matching ownership witness")
			require.ErrorContains(t, new(copyowner.Bindings).Record(target, key, name), "matching ownership witness")
		})
	}
}

func TestWritableMarkerWriteFailureRecovers(t *testing.T) {
	if os.Getenv("TALOS_WRITABLE_MARKER_PROBE_CHILD") != "1" {
		command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestWritableMarkerWriteFailureRecovers$", "-test.timeout=15s", "-test.v")

		command.Env = append(os.Environ(), "TALOS_WRITABLE_MARKER_PROBE_CHILD=1")

		output, err := command.CombinedOutput()
		require.NoError(t, err, "%s", output)

		return
	}

	target := t.TempDir()
	private := filepath.Join(target, ".copy-on-use")
	require.NoError(t, os.Mkdir(private, 0o700))

	stage := ".copy-on-use/probe-stage"
	require.NoError(t, os.WriteFile(filepath.Join(target, stage), []byte("already-converted-private-payload"), 0o600))

	root, err := os.OpenRoot(target)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })

	var limits syscall.Rlimit
	require.NoError(t, syscall.Getrlimit(syscall.RLIMIT_FSIZE, &limits))

	// Confine the write fault to this subprocess, after the payload exists.
	signal.Ignore(syscall.SIGXFSZ)

	limited := limits
	limited.Cur = 0
	require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limited))

	writeErr := copyowner.Publish(root, stage, "image-probe.raw", ".copy-on-use/owned-probe", "raw")
	restoreErr := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limits)
	require.NoError(t, restoreErr)
	require.ErrorIs(t, writeErr, syscall.EFBIG)

	// Capacity recovery must permit publication; it must not strand a partial
	// metadata file under the final ownership name and fail forever with EEXIST.
	require.NoError(t, copyowner.Publish(root, stage, "image-probe.raw", ".copy-on-use/owned-probe", "raw"))
}

func TestImageOwnershipStagesInsidePrivateDirectory(t *testing.T) {
	target := t.TempDir()
	root, err := os.OpenRoot(target)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	require.NoError(t, root.Mkdir(".copy-on-use", 0o700))

	stage := ".guest.raw.0123456789abcdef.staged"
	require.NoError(t, os.WriteFile(filepath.Join(target, stage), []byte("private image payload"), 0o600))
	markerStage := filepath.Join(".copy-on-use", filepath.Base(stage)+".owner")
	require.NoError(t, root.Mkdir(markerStage, 0o700))
	require.Error(t, copyowner.Publish(root, stage, "guest.raw", ".copy-on-use/owned-probe", "raw"))
	require.NoFileExists(t, filepath.Join(target, "guest.raw"))
	require.NoFileExists(t, filepath.Join(target, stage+".owner"))
}

func cleanupFixture(t *testing.T) string {
	t.Helper()
	target := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(target, ".copy-on-use"), 0o700))

	return target
}

func writeCleanupOwner(t *testing.T, target, key, name string) string {
	t.Helper()

	path := filepath.Join(target, ".copy-on-use", fmt.Sprintf("owner-%x", sha256.Sum256([]byte(name))))
	info, err := os.Lstat(filepath.Join(target, copyowner.Witness(name, false)))
	require.NoError(t, err)

	stat := info.Sys().(*syscall.Stat_t)
	require.NoError(t, os.WriteFile(path, []byte(fmt.Sprintf("%s\n%s\n%d\n", key, name, stat.Ino)), 0o600))

	return path
}

func TestCollectCopiesRetiresOnlyUnwantedWitnessAndOwnerAfterPublicDeletion(t *testing.T) {
	target := cleanupFixture(t)
	key := strings.Repeat("a", 64)
	name := "media.raw"
	witness := filepath.Join(target, copyowner.Witness(name, false))
	require.NoError(t, os.WriteFile(witness, []byte("bytes"), 0o600))
	owner := writeCleanupOwner(t, target, key, name)
	require.NoError(t, copyowner.Collect(context.Background(), target, copyowner.Retention{Bindings: map[string]struct{}{key: {}}}))
	require.FileExists(t, witness)
	require.FileExists(t, owner)
	require.NoError(t, copyowner.Collect(context.Background(), target, copyowner.Retention{}))
	require.NoFileExists(t, witness)
	require.NoFileExists(t, owner)
	require.NoError(t, copyowner.Collect(context.Background(), target, copyowner.Retention{}))
}

func TestCollectCopiesKeepsPublicPayloadAndBinding(t *testing.T) {
	target := cleanupFixture(t)
	name := "guest.raw"
	key := strings.Repeat("b", 64)
	public := filepath.Join(target, name)
	require.NoError(t, os.WriteFile(public, []byte("guest"), 0o600))
	require.NoError(t, os.Link(public, filepath.Join(target, copyowner.Witness(name, false))))
	require.NoError(t, new(copyowner.Bindings).Record(target, key, name))
	require.NoError(t, copyowner.Collect(context.Background(), target, copyowner.Retention{}))
	require.FileExists(t, public)
	require.FileExists(t, filepath.Join(target, copyowner.Witness(name, false)))
	require.FileExists(t, filepath.Join(target, ".copy-on-use", "binding-"+key))
}

func TestCollectCopiesPreservesReplacedWitnessAndOwner(t *testing.T) {
	target := cleanupFixture(t)
	name, key := "replaced.raw", strings.Repeat("c", 64)
	witness := filepath.Join(target, copyowner.Witness(name, false))
	require.NoError(t, os.WriteFile(witness, []byte("original"), 0o600))
	owner := writeCleanupOwner(t, target, key, name)
	require.NoError(t, os.Link(witness, filepath.Join(target, ".old-witness-inode")))
	require.NoError(t, os.Remove(witness))
	require.NoError(t, os.WriteFile(witness, []byte("foreign"), 0o600))
	require.NoError(t, copyowner.Collect(context.Background(), target, copyowner.Retention{}))
	require.FileExists(t, owner)

	// The owner record proves only the original inode; a replacement is never unlinked.
	data, err := os.ReadFile(witness)
	require.NoError(t, err)
	require.Equal(t, "foreign", string(data))
}

func TestCollectCopiesKeepsWantedBindingWitnessOnly(t *testing.T) {
	target := cleanupFixture(t)
	name, key := "legacy.iso", strings.Repeat("d", 64)
	witness := filepath.Join(target, copyowner.Witness(name, false))
	require.NoError(t, os.WriteFile(witness, []byte("legacy"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".copy-on-use", "binding-"+key), []byte(name), 0o600))
	require.NoError(t, copyowner.Collect(context.Background(), target, copyowner.Retention{Bindings: map[string]struct{}{key: {}}}))
	require.FileExists(t, witness)
}

func TestCollectCopiesProtectsActiveStageDestination(t *testing.T) {
	target := cleanupFixture(t)
	name, key := "inflight.iso", strings.Repeat("e", 64)
	witness := filepath.Join(target, copyowner.Witness(name, false))
	require.NoError(t, os.WriteFile(witness, []byte("bytes"), 0o600))
	owner := writeCleanupOwner(t, target, key, name)
	release := copyowner.ProtectStage(target, copyowner.StageName(name))
	require.NoError(t, copyowner.Collect(context.Background(), target, copyowner.Retention{}))
	require.FileExists(t, witness)
	release()
	require.NoError(t, copyowner.Collect(context.Background(), target, copyowner.Retention{}))
	require.NoFileExists(t, witness)
	require.NoFileExists(t, owner)
}

func TestCollectCopiesRetiresMissingWritableBindingAndMarker(t *testing.T) {
	target := cleanupFixture(t)
	name, key := "gone.img", strings.Repeat("f", 64)
	public := filepath.Join(target, name)
	require.NoError(t, os.WriteFile(public, []byte("guest bytes"), 0o600))

	root, err := os.OpenRoot(target)
	require.NoError(t, err)
	owner, err := copyowner.ClaimImageOwner(root, name)
	require.NoError(t, err)

	marker := copyowner.Witness(name, true)
	require.NoError(t, os.WriteFile(filepath.Join(target, marker), []byte(copyowner.ImageMarker("raw", owner)), 0o600))
	require.NoError(t, root.Close())

	binding := filepath.Join(target, ".copy-on-use", "binding-"+key)
	require.NoError(t, os.WriteFile(binding, []byte(name), 0o600))
	require.NoError(t, os.Remove(public))
	require.NoError(t, copyowner.Collect(context.Background(), target, copyowner.Retention{Bindings: map[string]struct{}{key: {}}}))
	require.NoFileExists(t, filepath.Join(target, marker))
	require.NoFileExists(t, binding)
}

func TestFailedWritableReplacementKeepsMarkerForCollectorRetry(t *testing.T) {
	for _, fault := range []string{"missing payload stage", "metadata stage collision"} {
		t.Run(fault, func(t *testing.T) {
			target, marker, binding, key := missingWritableCleanupFixture(t)
			previous, err := os.ReadFile(marker)
			require.NoError(t, err)
			root, err := os.OpenRoot(target)
			require.NoError(t, err)

			defer root.Close() //nolint:errcheck

			stage := ".replacement.0000000000000001.staged"
			if fault == "metadata stage collision" {
				require.NoError(t, os.WriteFile(filepath.Join(target, stage), []byte("new bytes"), 0o600))
				require.NoError(t, os.WriteFile(filepath.Join(target, ".copy-on-use", stage+".owner"), []byte("foreign metadata stage"), 0o600))
			}

			require.Error(t, copyowner.ClaimImageMarker(root, stage, "deleted-writable.img", copyowner.Witness("deleted-writable.img", true), "raw"))

			got, err := os.ReadFile(marker)
			require.NoError(t, err)
			require.Equal(t, previous, got, "failed replacement must retain the prior marker")
			require.FileExists(t, binding)
			require.NoError(t, copyowner.Collect(t.Context(), target, copyowner.Retention{Bindings: map[string]struct{}{key: {}}}))
			require.NoFileExists(t, binding)
			require.NoFileExists(t, marker)
		})
	}
}

func TestWritableReplacementAtomicallyPublishesKnownMarker(t *testing.T) {
	target, marker, _, _ := missingWritableCleanupFixture(t)
	previous, err := os.ReadFile(marker)
	require.NoError(t, err)
	root, err := os.OpenRoot(target)
	require.NoError(t, err)

	defer root.Close() //nolint:errcheck

	stage := ".replacement.0000000000000002.staged"
	require.NoError(t, os.WriteFile(filepath.Join(target, stage), []byte("new bytes"), 0o600))
	require.NoError(t, copyowner.ClaimImageMarker(root, stage, "deleted-writable.img", copyowner.Witness("deleted-writable.img", true), "raw"))

	updated, err := os.ReadFile(marker)
	require.NoError(t, err)
	require.NotEqual(t, previous, updated)

	file, err := root.Open(stage)
	require.NoError(t, err)
	match, err := copyowner.ImageFileMatchesMarker(root, copyowner.Witness("deleted-writable.img", true), file)
	require.NoError(t, file.Close())
	require.NoError(t, err)
	require.True(t, match)
}

func TestWritableReplacementPreservesForeignWitness(t *testing.T) {
	target := cleanupFixture(t)
	root, err := os.OpenRoot(target)
	require.NoError(t, err)

	defer root.Close() //nolint:errcheck

	stage, name := ".replacement.0000000000000003.staged", "foreign.img"
	require.NoError(t, os.WriteFile(filepath.Join(target, stage), []byte("new bytes"), 0o600))

	witness := copyowner.Witness(name, true)
	require.NoError(t, os.WriteFile(filepath.Join(target, witness), []byte("foreign witness"), 0o600))
	require.Error(t, copyowner.ClaimImageMarker(root, stage, name, witness, "raw"))
	got, err := os.ReadFile(filepath.Join(target, witness))
	require.NoError(t, err)
	require.Equal(t, "foreign witness", string(got))
}

func missingWritableCleanupFixture(t *testing.T) (target, marker, binding, key string) {
	t.Helper()

	target = cleanupFixture(t)
	name := "deleted-writable.img"
	key = strings.Repeat("f", 64)
	public := filepath.Join(target, name)
	require.NoError(t, os.WriteFile(public, []byte("guest bytes"), 0o600))

	root, err := os.OpenRoot(target)
	require.NoError(t, err)
	owner, err := copyowner.ClaimImageOwner(root, name)
	require.NoError(t, err)

	marker = filepath.Join(target, copyowner.Witness(name, true))
	require.NoError(t, os.WriteFile(marker, []byte(copyowner.ImageMarker("raw", owner)), 0o600))
	require.NoError(t, root.Close())

	binding = filepath.Join(target, ".copy-on-use", "binding-"+key)
	require.NoError(t, os.WriteFile(binding, []byte(name), 0o600))
	require.NoError(t, os.Remove(public))

	return target, marker, binding, key
}

func TestCollectCopiesRetiresWritableBindingBeforeMarkerOnBindingSyncFailure(t *testing.T) {
	target, marker, binding, key := missingWritableCleanupFixture(t)
	keep := copyowner.Retention{Bindings: map[string]struct{}{key: {}}}
	failure := fmt.Errorf("binding directory sync interrupted")
	faulted := false

	syncBarrier := func(root *os.Root, dir string) error {
		if !faulted && !fileExists(binding) && fileExists(marker) {
			faulted = true

			return failure
		}

		return copyowner.Sync(root, dir)
	}

	require.ErrorIs(t, copyowner.Collector{Sync: syncBarrier}.Collect(t.Context(), target, keep), failure)
	require.True(t, faulted, "fault must occur after binding unlink but before marker unlink")
	require.NoFileExists(t, binding)
	require.FileExists(t, marker, "the marker must remain as retry evidence")
	require.NoError(t, copyowner.Collect(t.Context(), target, keep))
	require.NoFileExists(t, binding)
	require.NoFileExists(t, marker)
}

func TestCollectCopiesRetriesWritableMarkerSyncFailure(t *testing.T) {
	target, marker, binding, key := missingWritableCleanupFixture(t)
	keep := copyowner.Retention{Bindings: map[string]struct{}{key: {}}}
	failure := fmt.Errorf("marker directory sync interrupted")
	faulted := false

	syncBarrier := func(root *os.Root, dir string) error {
		if !faulted && !fileExists(marker) {
			faulted = true

			return failure
		}

		return copyowner.Sync(root, dir)
	}

	require.ErrorIs(t, copyowner.Collector{Sync: syncBarrier}.Collect(t.Context(), target, keep), failure)
	require.True(t, faulted, "marker sync must follow binding retirement")
	require.NoFileExists(t, binding)
	require.NoFileExists(t, marker)
	require.NoError(t, copyowner.Collect(t.Context(), target, keep))
}

func TestCollectCopiesStandaloneMarkerWaitsForBindingBarrier(t *testing.T) {
	target, marker, binding, key := missingWritableCleanupFixture(t)
	require.NoError(t, os.Remove(binding)) // Simulate a crash after binding unlink, before its sync.

	keep := copyowner.Retention{Bindings: map[string]struct{}{key: {}}}
	failure := fmt.Errorf("standalone binding barrier interrupted")
	faulted := false

	syncBarrier := func(root *os.Root, dir string) error {
		if !faulted {
			faulted = true

			return failure
		}

		return copyowner.Sync(root, dir)
	}

	require.ErrorIs(t, copyowner.Collector{Sync: syncBarrier}.Collect(t.Context(), target, keep), failure)
	require.True(t, faulted)
	require.FileExists(t, marker, "a standalone marker must survive an uncommitted binding unlink")
	require.NoError(t, copyowner.Collect(t.Context(), target, keep))
	require.NoFileExists(t, marker)
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)

	return err == nil
}

func TestCollectCopiesPreservesUnknownMarkerName(t *testing.T) {
	target := cleanupFixture(t)
	name := "foreign.raw"
	public := filepath.Join(target, name)
	require.NoError(t, os.WriteFile(public, []byte("foreign data"), 0o600))

	root, err := os.OpenRoot(target)
	require.NoError(t, err)
	owner, err := copyowner.ClaimImageOwner(root, name)
	require.NoError(t, err)
	require.NoError(t, root.Close())

	unknown := filepath.Join(target, ".copy-on-use", "image-not-a-managed-hash")
	require.NoError(t, os.WriteFile(unknown, []byte(copyowner.ImageMarker("raw", owner)), 0o600))
	require.NoError(t, copyowner.Collect(t.Context(), target, copyowner.Retention{}))
	require.FileExists(t, unknown)
}

func TestCollectCopiesSweepsIdlePrivateStages(t *testing.T) {
	target := cleanupFixture(t)
	private := filepath.Join(target, ".copy-on-use")
	stale := filepath.Join(private, ".retired.raw.0123456789abcdef.staged.owner")
	binding := filepath.Join(private, ".binding-"+strings.Repeat("a", 64)+".0123456789abcdef.staged")
	activeName := ".live.raw.0123456789abcdef.staged"

	active := filepath.Join(private, activeName+".owner")
	for _, path := range []string{stale, binding, active} {
		require.NoError(t, os.WriteFile(path, []byte("private stage"), 0o600))
	}

	release := copyowner.ProtectStage(target, activeName)
	t.Cleanup(release)
	require.NoError(t, copyowner.Collect(t.Context(), target, copyowner.Retention{}))
	require.NoFileExists(t, stale)
	require.NoFileExists(t, binding)
	require.FileExists(t, active)
}

func TestCollectCopiesRetriesMissingWitnessBarrier(t *testing.T) {
	target := cleanupFixture(t)
	name, key := "retired.iso", strings.Repeat("a", 64)
	witness := filepath.Join(target, copyowner.Witness(name, false))
	require.NoError(t, os.WriteFile(witness, []byte("payload"), 0o600))
	owner := writeCleanupOwner(t, target, key, name)
	require.NoError(t, os.Remove(witness))

	failure := fmt.Errorf("directory sync interrupted")
	syncBarrier := func(*os.Root, string) error {
		require.FileExists(t, owner, "ownership proof must survive until the witness unlink is durable, including a retry")

		return failure
	}
	require.ErrorIs(t, copyowner.Collector{Sync: syncBarrier}.Collect(t.Context(), target, copyowner.Retention{}), failure)
	require.FileExists(t, owner)
	require.NoError(t, copyowner.Collect(t.Context(), target, copyowner.Retention{}))
	require.NoFileExists(t, owner)
}

func TestCopyOwnerRejectsDifferentStagedInode(t *testing.T) {
	target := cleanupFixture(t)
	root, err := os.OpenRoot(target)
	require.NoError(t, err)

	defer root.Close() //nolint:errcheck

	name, key := "same.iso", strings.Repeat("a", 64)
	witness := copyowner.Witness(name, false)
	first := copyowner.OwnerStage{Root: root, Name: name, Key: key, Witness: witness, Staged: ".copy-on-use/first.staged"}
	second := copyowner.OwnerStage{Root: root, Name: name, Key: key, Witness: witness, Staged: ".copy-on-use/second.staged"}

	for _, op := range []copyowner.OwnerStage{first, second} {
		require.NoError(t, os.WriteFile(filepath.Join(target, op.Staged), []byte(op.Staged), 0o600))
	}

	require.NoError(t, first.PublishOwner())
	// Once the first stage holds the witness, another inode cannot take over its owner record.
	require.NoError(t, root.Link(first.Staged, witness))
	require.Error(t, second.PublishOwner())

	firstInfo, err := root.Lstat(first.Staged)
	require.NoError(t, err)
	require.True(t, copyowner.OwnerMatches(root, key, name, firstInfo))
}

func TestCopyBindingDoesNotGuessBetweenRecoveryOwners(t *testing.T) {
	for _, missingWitness := range []bool{false, true} {
		t.Run(fmt.Sprintf("missing-witness-%t", missingWitness), func(t *testing.T) {
			target := cleanupFixture(t)
			key := strings.Repeat("a", 64)

			for _, name := range []string{"old.iso", "new.iso"} {
				path := filepath.Join(target, copyowner.Witness(name, false))
				require.NoError(t, os.WriteFile(path, []byte(name), 0o400))
				writeCleanupOwner(t, target, key, name)

				if missingWitness && name == "new.iso" {
					require.NoError(t, os.Remove(path))
				}
			}

			got, err := copyowner.ReadBinding(target, key)
			require.NoError(t, err)
			require.Empty(t, got, "multiple owned generations require the source; absence of a witness does not authorize an older generation")
		})
	}
}

func TestCopyBindingSelectsRecoverableImmutableWitness(t *testing.T) {
	for _, kind := range []string{"immutable", "immutable-unbound", "foreign-public", "wrong-owner", "missing-owner", "writable-marker"} {
		t.Run(kind, func(t *testing.T) {
			target := t.TempDir()
			key := strings.Repeat("a", 64)
			name := "installed.iso"

			require.NoError(t, os.Mkdir(filepath.Join(target, ".copy-on-use"), 0o700))

			witness := copyowner.Witness(name, false)
			require.NoError(t, os.WriteFile(filepath.Join(target, witness), []byte("immutable recovery bytes"), 0o400))
			info, err := os.Stat(filepath.Join(target, witness))
			require.NoError(t, err)

			ownerKey := key
			if kind == "wrong-owner" {
				ownerKey = strings.Repeat("b", 64)
			}

			if kind != "missing-owner" {
				owner := fmt.Sprintf("%s\n%s\n%s\n", ownerKey, name, inode(info))
				require.NoError(t, os.WriteFile(filepath.Join(target, copyowner.OwnerPath(name)), []byte(owner), 0o600))
			}

			binding, err := copyowner.BindingPath(key)
			require.NoError(t, err)

			if kind != "immutable-unbound" {
				require.NoError(t, os.WriteFile(filepath.Join(target, binding), []byte(name), 0o600))
			}

			if kind == "foreign-public" {
				require.NoError(t, os.WriteFile(filepath.Join(target, name), []byte("foreign bytes"), 0o600))
			}

			if kind == "writable-marker" {
				require.NoError(t, os.WriteFile(filepath.Join(target, copyowner.Witness(name, true)), []byte("writable metadata"), 0o600))
			}

			got, err := copyowner.ReadBinding(target, key)
			if kind == "immutable" || kind == "immutable-unbound" {
				require.NoError(t, err)
				require.Equal(t, name, got)
			} else {
				require.Error(t, err)
				require.Empty(t, got)
			}

			if kind == "foreign-public" {
				data, readErr := os.ReadFile(filepath.Join(target, name))
				require.NoError(t, readErr)
				require.Equal(t, "foreign bytes", string(data))
			} else {
				require.NoFileExists(t, filepath.Join(target, name), "binding lookup must not publish payloads")
			}
		})
	}
}

func imageOwnerFixtureToken(t *testing.T, path string) string {
	t.Helper()

	owner := uuid.NewString()
	require.NoError(t, unix.Setxattr(path, "user.talos.image-owner", []byte(owner), unix.XATTR_CREATE))

	return owner
}

func inode(info os.FileInfo) string {
	return strconv.FormatUint(info.Sys().(*syscall.Stat_t).Ino, 10)
}

func TestImageOwnerUnsupportedFilesystemIsPermanent(t *testing.T) {
	// procfs has no user extended attributes.
	file, err := os.Open("/proc/self/status")
	require.NoError(t, err)

	defer file.Close() //nolint:errcheck

	_, err = copyowner.ImageFileMatches(copyowner.ImageMarker("raw", uuid.NewString()), "raw", file)

	var validation *qemuimg.ValidationError

	require.ErrorAs(t, err, &validation)
	require.True(t, errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP), "%v", err)
	require.True(t, qemuimg.IsTerminal(err))
}

func TestMalformedLongOwnerRecordIsNeverSelected(t *testing.T) {
	target := cleanupFixture(t)
	key, name := strings.Repeat("a", 64), strings.Repeat("n", 255)
	path := filepath.Join(target, copyowner.OwnerPath(name))

	require.NoError(t, os.WriteFile(path, []byte(key+"\n"+name+"\n1:2\nEXTRA"), 0o600))

	selected, err := copyowner.ReadBinding(target, key)
	require.NoError(t, err)
	require.Empty(t, selected, "a malformed owner record selects no recovery copy")

	require.NoError(t, copyowner.Collect(t.Context(), target, copyowner.Retention{}))
	require.FileExists(t, path, "unknown private entries are left alone")
}

func TestLongCopyOwnerPublicationRecoveryAndCleanup(t *testing.T) {
	target := cleanupFixture(t)
	root, err := os.OpenRoot(target)
	require.NoError(t, err)

	defer root.Close() //nolint:errcheck

	name, key := strings.Repeat("n", 251)+".iso", strings.Repeat("a", 64)
	stage := copyowner.OwnerStage{
		Root:    root,
		Name:    name,
		Key:     key,
		Staged:  ".copy-on-use/long-stage",
		Witness: copyowner.Witness(name, false),
	}

	require.NoError(t, os.WriteFile(filepath.Join(target, stage.Staged), []byte("long-name payload"), 0o600))
	require.NoError(t, stage.PublishOwner())
	require.NoError(t, copyowner.Publish(root, stage.Staged, name, stage.Witness, ""))

	info, err := root.Lstat(name)
	require.NoError(t, err)
	require.True(t, copyowner.OwnerMatches(root, key, name, info), "the owner record fits a maximum-length name")

	require.NoError(t, os.Remove(filepath.Join(target, name)))
	require.NoError(t, copyowner.Collect(t.Context(), target, copyowner.Retention{Bindings: map[string]struct{}{key: {}}}))
	require.FileExists(t, filepath.Join(target, stage.Witness), "wanted copy must remain recoverable")

	recovered, err := copyowner.Recover(root, name, stage.Witness)
	require.NoError(t, err)
	require.True(t, recovered)
	require.FileExists(t, filepath.Join(target, name))

	require.NoError(t, os.Remove(filepath.Join(target, name)))
	require.NoError(t, copyowner.Collect(t.Context(), target, copyowner.Retention{}))
	require.NoFileExists(t, filepath.Join(target, stage.Witness))
	require.NoFileExists(t, filepath.Join(target, copyowner.OwnerPath(name)))
}
