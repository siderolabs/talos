// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage // tests exercise the unexported nvmeFlasher flow and the registry.
package flash

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDevice is an in-memory nvmeDevice.
type fakeDevice struct {
	downloads [][]byte
	commits   int
	closes    int

	downloadErr error
	commitErr   error
}

func (f *fakeDevice) DownloadFirmware(_ context.Context, image []byte) error {
	f.downloads = append(f.downloads, image)

	return f.downloadErr
}

func (f *fakeDevice) Commit(context.Context) error {
	f.commits++

	return f.commitErr
}

func (f *fakeDevice) Close() error {
	f.closes++

	return nil
}

func newFlasherFor(dev *fakeDevice, openErr error) nvmeFlasher {
	return nvmeFlasher{open: func(string) (nvmeDevice, error) {
		if openErr != nil {
			return nil, openErr
		}

		return dev, nil
	}}
}

func TestNvmeFlasherFlash(t *testing.T) {
	t.Parallel()

	dev := &fakeDevice{}
	flasher := newFlasherFor(dev, nil)

	require.NoError(t, flasher.Flash(context.Background(), Target{DevPath: "/dev/nvme0"}, []byte("image")))

	require.Len(t, dev.downloads, 1)
	assert.Equal(t, []byte("image"), dev.downloads[0])
	assert.Equal(t, 1, dev.commits)
	assert.Equal(t, 1, dev.closes)
}

func TestNvmeFlasherFlashDownloadError(t *testing.T) {
	t.Parallel()

	dev := &fakeDevice{downloadErr: errors.New("download failed")}
	flasher := newFlasherFor(dev, nil)

	err := flasher.Flash(context.Background(), Target{DevPath: "/dev/nvme0"}, []byte("image"))
	require.ErrorIs(t, err, dev.downloadErr)
	assert.Equal(t, 1, dev.closes) // device closed even on failure
	assert.Zero(t, dev.commits)    // commit never reached
}

func TestNvmeFlasherFlashCommitError(t *testing.T) {
	t.Parallel()

	dev := &fakeDevice{commitErr: errors.New("commit failed")}
	flasher := newFlasherFor(dev, nil)

	err := flasher.Flash(context.Background(), Target{DevPath: "/dev/nvme0"}, []byte("image"))
	require.ErrorIs(t, err, dev.commitErr)
	assert.Equal(t, 1, dev.commits)
	assert.Equal(t, 1, dev.closes)
}

func TestNvmeFlasherFlashOpenError(t *testing.T) {
	t.Parallel()

	openErr := errors.New("no such device")
	dev := &fakeDevice{}
	flasher := newFlasherFor(dev, openErr)

	err := flasher.Flash(context.Background(), Target{DevPath: "/dev/nvme0"}, []byte("image"))
	require.ErrorIs(t, err, openErr)
	assert.Zero(t, dev.closes)
	assert.Empty(t, dev.downloads)
	assert.Zero(t, dev.commits)
}

func TestNvmeFlasherFlashCanceledContext(t *testing.T) {
	t.Parallel()

	dev := &fakeDevice{}
	flasher := newFlasherFor(dev, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := flasher.Flash(ctx, Target{DevPath: "/dev/nvme0"}, []byte("image"))
	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, dev.downloads)
	assert.Zero(t, dev.commits)
	assert.Zero(t, dev.closes)
}

func TestForUnknownProtocol(t *testing.T) {
	t.Parallel()

	f, ok := For("com.example.nosuchprotocol")
	assert.False(t, ok)
	assert.Nil(t, f)
	assert.False(t, Supported("com.example.nosuchprotocol"))
}

func TestNVMeFlasherRegistered(t *testing.T) {
	t.Parallel()

	f, ok := For(ProtocolNVMe)
	require.True(t, ok)
	assert.NotNil(t, f)
	assert.True(t, Supported(ProtocolNVMe))
}

func TestRegisterDuplicatePanics(t *testing.T) {
	assert.Panics(t, func() {
		Register(ProtocolNVMe, nvmeFlasher{})
	})
}

func TestRegisterAndFor(t *testing.T) {
	dev := &fakeDevice{}
	Register("test.flash.example", newFlasherFor(dev, nil))

	f, ok := For("test.flash.example")
	require.True(t, ok)

	require.NoError(t, f.Flash(context.Background(), Target{DevPath: "/dev/x"}, []byte("fw")))
	require.Len(t, dev.downloads, 1)
	assert.Equal(t, 1, dev.commits)
}
