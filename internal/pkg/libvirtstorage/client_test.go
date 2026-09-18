// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package libvirtstorage_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"testing/synctest"
	"time"

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/internal/pkg/libvirtstorage"
)

const machine = "6b5baa32-47dd-4bcd-9c25-92870e3d43c9"

func TestUUID(t *testing.T) {
	t.Parallel()

	machineID := uuid.MustParse(machine)
	a := libvirtstorage.UUID(machineID, "images")
	// Pinned: changing the derivation orphans every existing pool definition.
	require.Equal(t, uuid.Version(8), a.Version())
	require.Equal(t, "79945509-76b8-81f6-94c6-26642a9091fc", a.String())

	require.Equal(t, a, libvirtstorage.UUID(machineID, "images"))
	require.NotEqual(t, a, libvirtstorage.UUID(uuid.MustParse("32c9e239-d898-4d20-a6e8-ea6c1c8071ea"), "images"))
	require.NotEqual(t, a, libvirtstorage.UUID(machineID, "other"))
}

type rpc struct {
	pools       []libvirt.StoragePool
	calls       []string
	target      string
	description string
	lookupErr   error
	listCalls   int
	active      bool
}

func (r *rpc) ConnectListAllStoragePools(int32, libvirt.ConnectListAllStoragePoolsFlags) ([]libvirt.StoragePool, uint32, error) {
	r.listCalls++

	return r.pools, uint32(len(r.pools)), nil
}

func (r *rpc) StoragePoolLookupByName(name string) (libvirt.StoragePool, error) {
	if r.lookupErr != nil {
		return libvirt.StoragePool{}, r.lookupErr
	}

	for _, pool := range r.pools {
		if pool.Name == name {
			return pool, nil
		}
	}

	return libvirt.StoragePool{}, libvirt.Error{Code: uint32(libvirt.ErrNoStoragePool), Message: "pool not found"}
}

func (r *rpc) StoragePoolGetXMLDesc(libvirt.StoragePool, libvirt.StorageXMLFlags) (string, error) {
	if r.description != "" {
		return r.description, nil
	}

	return (&libvirtxml.StoragePool{Type: "dir", Target: &libvirtxml.StoragePoolTarget{Path: r.target}}).Marshal()
}
func (*rpc) StoragePoolIsPersistent(libvirt.StoragePool) (int32, error) { return 1, nil }
func (r *rpc) StoragePoolSetAutostart(_ libvirt.StoragePool, value int32) error {
	if value != 0 {
		panic("autostart must remain disabled")
	}

	return nil
}

func (r *rpc) StoragePoolIsActive(libvirt.StoragePool) (int32, error) {
	if r.active {
		return 1, nil
	}

	return 0, nil
}

func (r *rpc) StoragePoolDestroy(libvirt.StoragePool) error {
	r.calls = append(r.calls, "stop")
	r.active = false

	return nil
}

func (r *rpc) StoragePoolDefineXML(text string, _ uint32) (libvirt.StoragePool, error) {
	var desc libvirtxml.StoragePool
	if err := desc.Unmarshal(text); err != nil {
		return libvirt.StoragePool{}, err
	}

	p := libvirt.StoragePool{Name: desc.Name, UUID: libvirt.UUID(uuid.MustParse(desc.UUID))}
	r.pools = []libvirt.StoragePool{p}
	r.target = desc.Target.Path
	r.calls = append(r.calls, "define")

	return p, nil
}

func (r *rpc) StoragePoolCreate(libvirt.StoragePool, libvirt.StoragePoolCreateFlags) error {
	r.calls = append(r.calls, "start")
	r.active = true

	return nil
}

func (r *rpc) StoragePoolUndefine(libvirt.StoragePool) error {
	r.calls = append(r.calls, "undefine")
	r.pools = nil

	return nil
}

func TestPersistentPoolLifecycle(t *testing.T) {
	t.Parallel()

	id := libvirtstorage.UUID(uuid.MustParse(machine), "images")

	pool := libvirtstorage.Pool{Name: "images", UUID: id}
	rpc := &rpc{}
	client := libvirtstorage.NewTestClient(rpc)
	prepare := func() error {
		rpc.calls = append(rpc.calls, "prepare")

		return nil
	}
	require.NoError(t, client.Ensure(pool, "/var/mnt/u-vms/images", prepare))
	require.Equal(t, []string{"prepare", "define", "start"}, rpc.calls)
	rpc.calls = nil

	require.NoError(t, client.Ensure(pool, "/var/mnt/u-vms/images", prepare))
	require.Equal(t, []string{"prepare"}, rpc.calls, "unchanged reapply must not stop or redefine")
	rpc.calls = nil

	require.NoError(t, client.Ensure(pool, "/var/mnt/x-new/images", prepare))
	require.Equal(t, []string{"stop", "prepare", "define", "start"}, rpc.calls)
	require.Equal(t, libvirt.UUID(id), rpc.pools[0].UUID)
	require.Equal(t, "/var/mnt/x-new/images", rpc.target)
	rpc.calls = nil

	require.NoError(t, client.Remove(pool))
	require.Equal(t, []string{"stop", "undefine"}, rpc.calls, "remove must not inspect or delete data")
	require.NoError(t, client.Remove(pool), "already absent removal is idempotent")
	require.NoError(t, client.Stop(pool), "already absent stop is idempotent")
	require.Zero(t, rpc.listCalls, "named pool operations must not enumerate unrelated pools")
}

func TestPoolLookupErrors(t *testing.T) {
	t.Parallel()

	pool := libvirtstorage.Pool{Name: "images", UUID: uuid.New()}
	for _, lookupErr := range []error{
		errors.New("connection closed"),
		libvirt.Error{Code: uint32(libvirt.ErrNoDomain), Message: "not a missing pool"},
	} {
		client := libvirtstorage.NewTestClient(&rpc{lookupErr: lookupErr})
		require.ErrorIs(t, client.Stop(pool), lookupErr)
		require.ErrorIs(t, client.Remove(pool), lookupErr)
		require.ErrorIs(t, client.Ensure(pool, "/unused", func() error {
			t.Fatal("a lookup failure must not be treated as permission to create a pool")

			return nil
		}), lookupErr)
	}
}

func TestPoolLookupWrappedAbsence(t *testing.T) {
	t.Parallel()

	rpc := &rpc{lookupErr: fmt.Errorf("lookup: %w", libvirt.Error{Code: uint32(libvirt.ErrNoStoragePool), Message: "pool not found"})}
	client := libvirtstorage.NewTestClient(rpc)
	pool := libvirtstorage.Pool{Name: "images", UUID: uuid.New()}
	require.NoError(t, client.Remove(pool))
	require.NoError(t, client.Stop(pool))
	require.Empty(t, rpc.calls)
}

func TestPoolXMLTarget(t *testing.T) {
	t.Parallel()

	pool := libvirtstorage.Pool{Name: "images", UUID: uuid.New()}
	target := "/var/mnt/vms/a&b<images>"
	rpc := &rpc{pools: []libvirt.StoragePool{{Name: pool.Name, UUID: libvirt.UUID(pool.UUID)}}, description: `<pool type="dir"/>`}
	client := libvirtstorage.NewTestClient(rpc)
	prepare := func() error { return nil }

	require.NoError(t, client.Ensure(pool, target, prepare), "missing XML target must trigger redefinition, not a nil dereference")
	require.Equal(t, target, rpc.target, "XML metacharacters must round-trip without changing the directory")
	rpc.description = ""
	rpc.calls = nil

	require.NoError(t, client.Ensure(pool, target, prepare))
	require.Empty(t, rpc.calls, "an escaped target must not cause perpetual redefinition")
}

func TestCollisionRefusedBeforePreparingDirectory(t *testing.T) {
	t.Parallel()

	id := libvirtstorage.UUID(uuid.MustParse(machine), "images")

	rpc := &rpc{pools: []libvirt.StoragePool{{Name: "images", UUID: libvirt.UUID(uuid.New())}}}
	client := libvirtstorage.NewTestClient(rpc)
	pool := libvirtstorage.Pool{Name: "images", UUID: id}
	require.ErrorContains(t, client.Ensure(pool, "/unused", func() error {
		t.Fatal("must not create directory")

		return nil
	}), "not owned")
	require.ErrorContains(t, client.Remove(pool), "not owned")
	require.ErrorContains(t, client.Stop(pool), "not owned")
	require.Empty(t, rpc.calls)
}

func TestHandshakeCancellationClosesTransport(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		client, server := net.Pipe()
		defer server.Close() //nolint:errcheck

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		sessionCtx, cancelSession := context.WithTimeout(ctx, time.Hour)
		defer cancelSession()

		read := make(chan struct{})

		go func() {
			var b [1]byte

			_, err := server.Read(b[:])
			assert.NoError(t, err)

			close(read)

			_, err = io.Copy(io.Discard, server)
			assert.NoError(t, err)
		}()

		done := make(chan error, 1)

		go func() { _, err := libvirtstorage.OpenConn(sessionCtx, client, cancelSession); done <- err }()

		<-read
		cancel()
		synctest.Wait()

		select {
		case err := <-done:
			require.Error(t, err)
		default:
			t.Error("context cancellation did not interrupt libvirt handshake")
			require.NoError(t, client.Close())
			<-done
		}

		synctest.Wait()
	})
}

func TestHandshakeExpiredDeadline(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		client, server := net.Pipe()
		defer server.Close() //nolint:errcheck

		ctx, cancel := context.WithTimeout(t.Context(), -time.Second)
		defer cancel()

		done := make(chan error, 1)

		go func() {
			_, err := libvirtstorage.OpenConn(ctx, client, cancel)
			done <- err
		}()

		synctest.Wait()

		select {
		case err := <-done:
			require.Error(t, err)
		default:
			t.Fatal("expired context did not interrupt libvirt handshake")
		}
	})
}
