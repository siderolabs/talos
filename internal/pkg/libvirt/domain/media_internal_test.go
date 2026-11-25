// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/digitalocean/go-libvirt"
	"github.com/stretchr/testify/require"
	"libvirt.org/go/libvirtxml"
)

// updateRPC serves only the device update of a medium change.
type updateRPC struct {
	domainRPC

	update func() error
	calls  int
}

func (r *updateRPC) DomainUpdateDeviceFlags(libvirt.Domain, string, libvirt.DomainDeviceModifyFlags) error {
	r.calls++

	return r.update()
}

func cdromChange() []mediumChange {
	drive := func(source string) *libvirtxml.DomainDisk {
		return &libvirtxml.DomainDisk{
			Device: "cdrom",
			Alias:  &libvirtxml.DomainAlias{Name: DeviceAlias("disk", "install")},
			Source: &libvirtxml.DomainDiskSource{File: &libvirtxml.DomainDiskSourceFile{File: source}},
		}
	}

	return []mediumChange{{current: drive("/lib/a.iso"), desired: drive("/lib/b.iso")}}
}

// A session which ran out before the medium change is not a guest refusing to release its tray.
func TestMediaChangeAfterSessionTimeoutIsNotARefusal(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancelCause(t.Context())
	watchdog := time.AfterFunc(time.Hour, func() {})
	watchdog.Stop()
	cancel(errSessionTimedOut)

	rpc := &updateRPC{update: func() error { return nil }}
	c := &client{rpc: rpc, ctx: ctx, watchdog: watchdog}

	err := c.changeMedia(libvirt.Domain{Name: "first"}, cdromChange(), definitionMetadata{}, definition{})
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrMediaChange), "%v", err)
	require.ErrorIs(t, err, errSessionTimedOut)
	require.Zero(t, rpc.calls, "no change is attempted on an ended session")
}

// A timeout which fires during the change is the guest not releasing its tray.
func TestMediaChangeTimeoutDuringTheChangeIsARefusal(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancelCause(t.Context())
	watchdog := time.AfterFunc(time.Hour, func() {})

	t.Cleanup(func() { watchdog.Stop() })

	rpc := &updateRPC{update: func() error {
		cancel(errSessionTimedOut)

		return errors.New("transport closed")
	}}
	c := &client{rpc: rpc, ctx: ctx, watchdog: watchdog}

	err := c.changeMedia(libvirt.Domain{Name: "first"}, cdromChange(), definitionMetadata{}, definition{})
	require.ErrorIs(t, err, ErrMediaChange)
	require.Equal(t, 1, rpc.calls)
}
