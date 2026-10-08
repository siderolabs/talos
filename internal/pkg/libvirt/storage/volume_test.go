// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"libvirt.org/go/libvirtxml"

	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
)

// qcow2Magic is what a qcow2 file starts with. The fake writes it so that a refresh can probe a
// file's format the way libvirt does, rather than trusting a name.
var qcow2Magic = []byte{'Q', 'F', 'I', 0xfb}

func (r *rpc) StoragePoolRefresh(libvirt.StoragePool, uint32) error {
	r.refreshes++
	r.known = map[string]struct{}{}

	entries, err := os.ReadDir(r.target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}

		r.known[entry.Name()] = struct{}{}
	}

	return nil
}

func (r *rpc) StorageVolLookupByName(p libvirt.StoragePool, name string) (libvirt.StorageVol, error) {
	if _, found := r.known[name]; !found {
		return libvirt.StorageVol{}, libvirt.Error{Code: uint32(libvirt.ErrNoStorageVol), Message: "volume not found"}
	}

	return libvirt.StorageVol{Pool: p.Name, Name: name, Key: name}, nil
}

// writeVolumeFile lays down what libvirt would: a sparse file, with a qcow2 header when asked for
// one, so that a refresh can probe its format the way libvirt does.
func writeVolumeFile(path, format string, capacity uint64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	if format == "qcow2" {
		if _, err = f.Write(qcow2Magic); err != nil {
			return err
		}
	}

	return f.Truncate(int64(capacity)) //nolint:gosec
}

func (r *rpc) StorageVolCreateXML(p libvirt.StoragePool, text string, flags libvirt.StorageVolCreateFlags) (libvirt.StorageVol, error) {
	var desc libvirtxml.StorageVolume
	if err := desc.Unmarshal(text); err != nil {
		return libvirt.StorageVol{}, err
	}

	if flags != 0 {
		panic("volume creation must pass no flags")
	}

	if desc.Allocation == nil || desc.Allocation.Value != 0 {
		panic("a volume must be created sparse, or the session expires mid-write")
	}

	r.volCalls = append(r.volCalls, "create:"+desc.Name+":"+desc.Target.Format.Type)

	if r.createErr != nil {
		return libvirt.StorageVol{}, r.createErr
	}

	if err := writeVolumeFile(filepath.Join(r.target, desc.Name), desc.Target.Format.Type, desc.Capacity.Value); err != nil {
		return libvirt.StorageVol{}, err
	}

	if r.known == nil {
		r.known = map[string]struct{}{}
	}

	r.known[desc.Name] = struct{}{}

	return libvirt.StorageVol{Pool: p.Name, Name: desc.Name, Key: desc.Name}, nil
}

func (r *rpc) StorageVolGetXMLDesc(vol libvirt.StorageVol, _ uint32) (string, error) {
	contents, err := os.ReadFile(filepath.Join(r.target, vol.Name))
	if err != nil {
		return "", err
	}

	format := "raw"
	if len(contents) >= len(qcow2Magic) && string(contents[:len(qcow2Magic)]) == string(qcow2Magic) {
		format = "qcow2"
	}

	return (&libvirtxml.StorageVolume{
		Name:   vol.Name,
		Target: &libvirtxml.StorageVolumeTarget{Format: &libvirtxml.StorageVolumeTargetFormat{Type: format}},
	}).Marshal()
}

func (r *rpc) StorageVolGetInfo(vol libvirt.StorageVol) (int8, uint64, uint64, error) {
	info, err := os.Stat(filepath.Join(r.target, vol.Name))
	if err != nil {
		return 0, 0, 0, err
	}

	return 0, uint64(info.Size()), uint64(info.Size()), nil //nolint:gosec
}

func (r *rpc) StorageVolGetPath(vol libvirt.StorageVol) (string, error) {
	return filepath.Join(r.target, vol.Name), nil
}

func (r *rpc) StorageVolResize(vol libvirt.StorageVol, capacity uint64, flags libvirt.StorageVolResizeFlags) error {
	if flags&libvirt.StorageVolResizeShrink != 0 {
		panic("a volume must never be shrunk")
	}

	r.volCalls = append(r.volCalls, "resize:"+vol.Name)

	if r.resizeErr != nil {
		return r.resizeErr
	}

	info, err := os.Stat(filepath.Join(r.target, vol.Name))
	if err != nil {
		return err
	}

	if capacity < uint64(info.Size()) { //nolint:gosec
		return libvirt.Error{Code: uint32(libvirt.ErrInvalidArg), Message: "shrinking is not supported"}
	}

	return os.Truncate(filepath.Join(r.target, vol.Name), int64(capacity)) //nolint:gosec
}

// newVolumeFixture builds a client over a pool whose directory really exists, because creation
// renames a staged file into place on the host rather than through libvirt.
func newVolumeFixture(t *testing.T) (*rpc, libvirtstorage.EvidenceClient, libvirtstorage.Pool) {
	t.Helper()

	id := libvirtstorage.UUID(uuid.MustParse(machine), "images")
	pool := libvirtstorage.Pool{Name: "images", UUID: id}
	r := &rpc{
		pools:  []libvirt.StoragePool{{Name: "images", UUID: libvirt.UUID(id)}},
		target: t.TempDir(),
		known:  map[string]struct{}{},
	}

	return r, libvirtstorage.NewTestClient(r), pool
}

func TestVolumeCreateAndLookup(t *testing.T) {
	t.Parallel()

	rpc, client, pool := newVolumeFixture(t)

	_, found, err := client.Volume(pool, "vm1__data.qcow2")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Equal(t, 1, rpc.refreshes, "an absent volume must be confirmed against a refreshed pool")

	volume, err := client.CreateVolume(pool, "vm1__data.qcow2", "qcow2", 64<<20, "", "")
	require.NoError(t, err)
	assert.Equal(t, "vm1__data.qcow2", volume.Name)
	assert.Equal(t, filepath.Join(rpc.target, "vm1__data.qcow2"), volume.Path)
	assert.Equal(t, "qcow2", volume.Format)
	assert.Equal(t, uint64(64<<20), volume.Capacity)

	// The staged name never survives, and the file is the one that was asked for.
	entries, err := os.ReadDir(rpc.target)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "vm1__data.qcow2", entries[0].Name())

	require.Len(t, rpc.volCalls, 1)
	assert.True(t, strings.HasSuffix(rpc.volCalls[0], ":qcow2"))
	assert.NotEqual(t, "create:vm1__data.qcow2:qcow2", rpc.volCalls[0],
		"a volume must be built under a staged name, so a crash cannot leave a half-written one in place")

	again, found, err := client.Volume(pool, "vm1__data.qcow2")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, volume, again)
}

type failingVolumeRead struct {
	*rpc
	step string
	err  error
}

func (r *failingVolumeRead) StorageVolLookupByName(p libvirt.StoragePool, name string) (libvirt.StorageVol, error) {
	if r.step == "lookup" {
		return libvirt.StorageVol{}, r.err
	}

	return r.rpc.StorageVolLookupByName(p, name)
}

func (r *failingVolumeRead) StoragePoolRefresh(p libvirt.StoragePool, flags uint32) error {
	if r.step == "refresh" {
		return r.err
	}

	return r.rpc.StoragePoolRefresh(p, flags)
}

func (r *failingVolumeRead) StorageVolGetPath(vol libvirt.StorageVol) (string, error) {
	if r.step == "path" {
		return "", r.err
	}

	return r.rpc.StorageVolGetPath(vol)
}

func (r *failingVolumeRead) StorageVolGetXMLDesc(vol libvirt.StorageVol, flags uint32) (string, error) {
	if r.step == "xml" {
		return "", r.err
	}

	if r.step == "malformed" {
		return "<volume>", nil
	}

	return r.rpc.StorageVolGetXMLDesc(vol, flags)
}

func (r *failingVolumeRead) StorageVolGetInfo(vol libvirt.StorageVol) (int8, uint64, uint64, error) {
	if r.step == "info" {
		return 0, 0, 0, r.err
	}

	return r.rpc.StorageVolGetInfo(vol)
}

func TestVolumeReadDistinguishesLostObservationFromInvalidVolume(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		step string
		err  error
	}{
		{name: "pool lookup", step: "pool lookup", err: context.DeadlineExceeded},
		{name: "lookup", step: "lookup", err: context.DeadlineExceeded},
		{name: "disconnected reply", step: "lookup", err: libvirt.Error{Code: uint32(libvirt.ErrNoConnect), Message: "disconnected"}},
		{name: "interrupted reply", step: "lookup", err: libvirt.ErrInterrupted},
		{name: "canceled reply", step: "lookup", err: context.Canceled},
		{name: "closed transport", step: "path", err: io.ErrClosedPipe},
		{name: "transport EOF", step: "xml", err: io.EOF},
		{name: "short transport read", step: "lookup", err: io.ErrUnexpectedEOF},
		{name: "network failure", step: "info", err: &net.OpError{Op: "read", Net: "unix", Err: net.ErrClosed}},
		{name: "refresh", step: "refresh", err: context.DeadlineExceeded},
		{name: "path", step: "path", err: context.DeadlineExceeded},
		{name: "xml", step: "xml", err: context.DeadlineExceeded},
		{name: "info", step: "info", err: context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, _, pool := newVolumeFixture(t)

			name := "data.raw"
			if tc.step != "refresh" {
				require.NoError(t, writeVolumeFile(filepath.Join(r.target, name), "raw", 1<<20))
				r.known[name] = struct{}{}
			}

			if tc.step == "pool lookup" {
				r.lookupErr = tc.err
			}

			client := libvirtstorage.NewTestClient(&failingVolumeRead{rpc: r, step: tc.step, err: tc.err})
			_, found, err := client.Volume(pool, name)
			require.False(t, found)
			require.ErrorIs(t, err, libvirtstorage.ErrVolumeObservationUnavailable)

			require.ErrorIs(t, err, tc.err)
		})
	}

	for _, tc := range []struct {
		name string
		step string
		err  error
	}{
		{name: "missing volume", step: "missing"},
		{name: "invalid lookup reply", step: "lookup", err: libvirt.Error{Code: uint32(libvirt.ErrInvalidArg), Message: "invalid"}},
		{name: "unsupported procedure", step: "lookup", err: libvirt.ErrUnsupported},
		{name: "unknown error", step: "path", err: errors.New("unexpected provider failure")},
		{name: "malformed description", step: "malformed"},
		{name: "wrong pool owner", step: "owner"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, _, pool := newVolumeFixture(t)
			if tc.step != "missing" {
				require.NoError(t, writeVolumeFile(filepath.Join(r.target, "data.raw"), "raw", 1<<20))
				r.known["data.raw"] = struct{}{}
			}

			if tc.step == "owner" {
				r.pools[0].UUID = libvirt.UUID{}
			}

			client := libvirtstorage.NewTestClient(&failingVolumeRead{rpc: r, step: tc.step, err: tc.err})
			_, found, err := client.Volume(pool, "data.raw")
			require.NotErrorIs(t, err, libvirtstorage.ErrVolumeObservationUnavailable)

			if tc.step == "missing" {
				require.NoError(t, err)
				require.False(t, found)
			} else {
				require.Error(t, err)

				if tc.err != nil {
					require.ErrorIs(t, err, tc.err)
				}
			}
		})
	}
}

func TestVolumeReadCompletedMalformedRPCReplyIsNotAnOutage(t *testing.T) {
	t.Parallel()

	raw, server := net.Pipe()
	defer raw.Close()    //nolint:errcheck
	defer server.Close() //nolint:errcheck

	served := make(chan error, 1)

	go func() {
		if err := serveHandshake(server); err != nil {
			served <- err

			return
		}

		call, err := readCall(server)
		if err != nil {
			served <- err

			return
		}

		if procedure := binary.BigEndian.Uint32(call[8:12]); procedure != 100 { // STORAGE_VOL_GET_PATH
			served <- fmt.Errorf("expected path procedure, got %d", procedure)

			return
		}

		// A complete successful RPC reply with no XDR string is a decode failure,
		// not a socket EOF or an unreceived reply.
		served <- replyCall(server, call, nil)
	}()

	sdk := libvirt.NewWithDialer(dialers.NewAlreadyConnected(raw))
	require.NoError(t, sdk.Connect())

	_, decodeErr := sdk.StorageVolGetPath(libvirt.StorageVol{Name: "data.raw"})

	require.NoError(t, <-served)
	require.Error(t, decodeErr)
	require.NotErrorIs(t, decodeErr, io.EOF)

	r, _, pool := newVolumeFixture(t)
	require.NoError(t, writeVolumeFile(filepath.Join(r.target, "data.raw"), "raw", 1<<20))
	r.known["data.raw"] = struct{}{}

	client := libvirtstorage.NewTestClient(&failingVolumeRead{rpc: r, step: "path", err: decodeErr})
	_, found, err := client.Volume(pool, "data.raw")
	require.False(t, found)
	require.ErrorIs(t, err, decodeErr)
	require.NotErrorIs(t, err, libvirtstorage.ErrVolumeObservationUnavailable)
}

type captureVolumeXML struct {
	*rpc
	createdXML string
}

func (r *captureVolumeXML) StorageVolCreateXML(pool libvirt.StoragePool, text string, flags libvirt.StorageVolCreateFlags) (libvirt.StorageVol, error) {
	r.createdXML = text

	return r.rpc.StorageVolCreateXML(pool, text, flags)
}

func TestVolumeCreateBackingStoreXML(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		backingFile   string
		backingFormat string
		wantFormat    string
	}{
		{name: "blank"},
		{name: "format without file", backingFormat: "qcow2"},
		{name: "default raw base", backingFile: `/var/mnt/images/base<&"'.raw`},
		{name: "explicit raw base", backingFile: "/var/mnt/images/base.raw", backingFormat: "raw", wantFormat: "raw"},
		{name: "qcow2 base", backingFile: "/var/mnt/images/base.qcow2", backingFormat: "qcow2", wantFormat: "qcow2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, _, pool := newVolumeFixture(t)
			capture := &captureVolumeXML{rpc: r}
			client := libvirtstorage.NewTestClient(capture)

			_, err := client.CreateVolume(pool, "overlay.qcow2", "qcow2", 1<<20, tc.backingFile, tc.backingFormat)
			require.NoError(t, err)

			var desc libvirtxml.StorageVolume
			require.NoError(t, desc.Unmarshal(capture.createdXML))

			if tc.backingFile == "" {
				assert.Nil(t, desc.BackingStore)
				assert.NotContains(t, capture.createdXML, "backingStore")

				return
			}

			require.NotNil(t, desc.BackingStore)
			assert.Equal(t, "qcow2", desc.Target.Format.Type, "overlay format is independent of base format")
			assert.Equal(t, tc.backingFile, desc.BackingStore.Path)

			if tc.wantFormat == "" {
				assert.Nil(t, desc.BackingStore.Format)
			} else {
				require.NotNil(t, desc.BackingStore.Format)
				assert.Equal(t, tc.wantFormat, desc.BackingStore.Format.Type)
			}

			if tc.name == "default raw base" {
				assert.NotContains(t, capture.createdXML, "base<&")
				assert.Contains(t, capture.createdXML, "base&lt;&amp;")
			}
		})
	}
}

// collisionOnCreate inserts a real destination after the initial libvirt lookup and
// after staging is complete, at the filesystem publication boundary.
type collisionOnCreate struct {
	*rpc
	name          string
	contents      []byte
	symlinkTarget string
}

func (r *collisionOnCreate) StorageVolCreateXML(p libvirt.StoragePool, text string, flags libvirt.StorageVolCreateFlags) (libvirt.StorageVol, error) {
	vol, err := r.rpc.StorageVolCreateXML(p, text, flags)
	if err != nil {
		return vol, err
	}

	if r.symlinkTarget != "" {
		return vol, os.Symlink(r.symlinkTarget, filepath.Join(r.target, r.name))
	}

	if err = os.WriteFile(filepath.Join(r.target, r.name), r.contents, 0o600); err != nil {
		return vol, err
	}

	return vol, nil
}

func TestVolumeCreateRefusesPublicationCollision(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"raw", "qcow2"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			r, _, pool := newVolumeFixture(t)
			name := "vm1__data." + format
			original := []byte("existing guest data")
			client := libvirtstorage.NewTestClient(&collisionOnCreate{rpc: r, name: name, contents: original})

			_, err := client.CreateVolume(pool, name, format, 1<<20, "", "")
			require.Error(t, err)
			assert.ErrorIs(t, err, os.ErrExist)

			contents, err := os.ReadFile(filepath.Join(r.target, name))
			require.NoError(t, err)
			assert.Equal(t, original, contents, "a late-arriving volume must retain its exact bytes")

			entries, err := os.ReadDir(r.target)
			require.NoError(t, err)
			require.Len(t, entries, 1, "a failed publication must remove its staging file")
			assert.Equal(t, name, entries[0].Name())
			assert.Equal(t, 1, r.refreshes, "failed publication must not refresh the pool")
		})
	}
}

func TestVolumeCreateRefusesPublicationSymlink(t *testing.T) {
	t.Parallel()

	r, _, pool := newVolumeFixture(t)
	outside := filepath.Join(t.TempDir(), "guest.raw")
	original := []byte("outside guest data")
	require.NoError(t, os.WriteFile(outside, original, 0o600))

	name := "vm1__data.raw"
	client := libvirtstorage.NewTestClient(&collisionOnCreate{rpc: r, name: name, symlinkTarget: outside})

	_, err := client.CreateVolume(pool, name, "raw", 1<<20, "", "")
	require.ErrorIs(t, err, os.ErrExist)

	contents, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, original, contents)

	link, err := os.Readlink(filepath.Join(r.target, name))
	require.NoError(t, err)
	assert.Equal(t, outside, link)

	entries, err := os.ReadDir(r.target)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no staged file may remain after a symlink collision")
}

func TestVolumeCreateRefusesAnExistingName(t *testing.T) {
	t.Parallel()

	_, client, pool := newVolumeFixture(t)

	_, err := client.CreateVolume(pool, "vm1__data.raw", "raw", 1<<20, "", "")
	require.NoError(t, err)

	_, err = client.CreateVolume(pool, "vm1__data.raw", "raw", 2<<20, "", "")
	require.ErrorContains(t, err, "already exists", "creation must never overwrite a guest's data")
}

// A volume written out of band -- or by this host before its last restart -- is invisible to libvirt
// until the pool is refreshed. It must still be found rather than created over.
func TestVolumeFoundAfterRefresh(t *testing.T) {
	t.Parallel()

	rpc, client, pool := newVolumeFixture(t)

	require.NoError(t, os.WriteFile(filepath.Join(rpc.target, "vm1__data.raw"), make([]byte, 4096), 0o600))

	volume, found, err := client.Volume(pool, "vm1__data.raw")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "raw", volume.Format)
	assert.Equal(t, uint64(4096), volume.Capacity)
	assert.Equal(t, 1, rpc.refreshes)

	_, err = client.CreateVolume(pool, "vm1__data.raw", "raw", 1<<20, "", "")
	require.ErrorContains(t, err, "already exists")
}

func TestResizeOperationEvidence(t *testing.T) {
	t.Parallel()

	r, client, pool := newVolumeFixture(t)
	lost := errors.New("lost reply")
	outcome, err := client.ResizeVolumeOperation(pool, "missing", 10)
	require.Equal(t, libvirtstorage.NoMutationSubmitted, outcome)
	require.Error(t, err)

	r.known["disk.raw"] = struct{}{}
	r.resizeErr = lost
	outcome, err = client.ResizeVolumeOperation(pool, "disk.raw", 10)
	require.ErrorIs(t, err, lost)
	require.Equal(t, libvirtstorage.Unknown, outcome)

	r.resizeErr = libvirt.Error{Code: uint32(libvirt.ErrOperationFailed), Message: "no resize"}
	outcome, err = client.ResizeVolumeOperation(pool, "disk.raw", 10)
	require.Error(t, err)
	require.Equal(t, libvirtstorage.Finished, outcome)
}

func TestVolumeResizeGrowsOnly(t *testing.T) {
	t.Parallel()

	_, client, pool := newVolumeFixture(t)

	_, err := client.CreateVolume(pool, "vm1__data.raw", "raw", 1<<20, "", "")
	require.NoError(t, err)

	require.NoError(t, client.ResizeVolume(pool, "vm1__data.raw", 2<<20))

	volume, found, err := client.Volume(pool, "vm1__data.raw")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, uint64(2<<20), volume.Capacity)

	// The shrink flag is never passed, so libvirt itself is the last line of defense.
	require.Error(t, client.ResizeVolume(pool, "vm1__data.raw", 1<<20))

	volume, _, err = client.Volume(pool, "vm1__data.raw")
	require.NoError(t, err)
	assert.Equal(t, uint64(2<<20), volume.Capacity, "a refused shrink must leave the volume alone")
}

func TestVolumeOperationsRefuseAForeignPool(t *testing.T) {
	t.Parallel()

	rpc, client, _ := newVolumeFixture(t)
	rpc.pools = []libvirt.StoragePool{{Name: "images", UUID: libvirt.UUID(uuid.New())}}
	pool := libvirtstorage.Pool{Name: "images", UUID: libvirtstorage.UUID(uuid.MustParse(machine), "images")}

	_, _, err := client.Volume(pool, "vm1__data.raw")
	require.ErrorContains(t, err, "not owned")

	_, err = client.CreateVolume(pool, "vm1__data.raw", "raw", 1<<20, "", "")
	require.ErrorContains(t, err, "not owned")

	require.ErrorContains(t, client.ResizeVolume(pool, "vm1__data.raw", 1<<20), "not owned")
	assert.Empty(t, rpc.volCalls)
}

// A create interrupted before the rename leaves a staged file. The next create sweeps it, and
// nothing else: without that, a pool directory collects one per interrupted or failed attempt.
func TestCreateVolumeSweepsStagedVolumes(t *testing.T) {
	t.Parallel()

	rpc, client, pool := newVolumeFixture(t)

	_, err := client.CreateVolume(pool, "vm1__data.raw", "raw", 1<<20, "", "")
	require.NoError(t, err)

	preserved := make([]string, 0, 14)

	preserved = append(preserved,
		".backup.staged",
		"..0123456789abcdef.staged",
		"...0123456789abcdef.staged",
		"....0123456789abcdef.staged",
		".data.raw..staged",
		".data.raw.0123456789abcde.staged",
		".data.raw.0123456789abcdef0.staged",
		".data.raw.0123456789abcdeg.staged",
		".data.raw.0123456789abcdeF.staged",
		"data.raw.0123456789abcdef.staged",
	)
	for _, name := range preserved {
		require.NoError(t, os.WriteFile(filepath.Join(rpc.target, name), []byte("operator data"), 0o600))
	}

	directory := ".directory.raw.0123456789abcdef.staged"
	require.NoError(t, os.Mkdir(filepath.Join(rpc.target, directory), 0o700))
	preserved = append(preserved, directory)

	outside := filepath.Join(t.TempDir(), "operator.raw")
	require.NoError(t, os.WriteFile(outside, []byte("operator target"), 0o600))

	symlink := ".linked.raw.0123456789abcdef.staged"
	require.NoError(t, os.Symlink(outside, filepath.Join(rpc.target, symlink)))

	staged := filepath.Join(rpc.target, ".vm2__data.raw.0123456789abcdef.staged")
	require.NoError(t, os.WriteFile(staged, nil, 0o600))

	_, err = client.CreateVolume(pool, "vm2__data.raw", "raw", 1<<20, "", "")
	require.NoError(t, err)

	entries, err := os.ReadDir(rpc.target)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	assert.ElementsMatch(t, append(preserved, symlink, "vm1__data.raw", "vm2__data.raw"), names,
		"only generated regular staged files may be swept")

	link, err := os.Readlink(filepath.Join(rpc.target, symlink))
	require.NoError(t, err)
	assert.Equal(t, outside, link)
	target, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, []byte("operator target"), target)

	for _, name := range preserved[:len(preserved)-1] {
		contents, readErr := os.ReadFile(filepath.Join(rpc.target, name))
		require.NoError(t, readErr)
		assert.Equal(t, []byte("operator data"), contents)
	}
}
