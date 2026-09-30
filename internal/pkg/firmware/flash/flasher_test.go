// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux && (amd64 || arm64 || loong64 || riscv64 || s390x)

//nolint:testpackage // tests exercise the unexported nvmeFlasher flow and the registry.
package flash

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDevice is an in-memory nvmeDevice.
type fakeDevice struct {
	// subsystem defaults to a name unique to the device, so parallel tests
	// don't serialize on each other.
	subsystem string

	downloads [][]byte
	commits   int
	closes    int

	activation  Activation
	downloadErr error
	commitErr   error
}

func (f *fakeDevice) Subsystem() string {
	if f.subsystem == "" {
		return fmt.Sprintf("fake-%p", f)
	}

	return f.subsystem
}

func (f *fakeDevice) DownloadFirmware(_ context.Context, image []byte) error {
	f.downloads = append(f.downloads, image)

	return f.downloadErr
}

func (f *fakeDevice) Commit(context.Context) (Activation, error) {
	f.commits++

	return f.activation, f.commitErr
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

	dev := &fakeDevice{activation: ActivationPowerCycle}
	flasher := newFlasherFor(dev, nil)

	activation, err := flasher.Flash(context.Background(), Target{DevPath: "/dev/nvme0"}, []byte("image"))
	require.NoError(t, err)
	assert.Equal(t, ActivationPowerCycle, activation)

	require.Len(t, dev.downloads, 1)
	assert.Equal(t, []byte("image"), dev.downloads[0])
	assert.Equal(t, 1, dev.commits)
	assert.Equal(t, 1, dev.closes)
}

func TestNvmeFlasherFlashDownloadError(t *testing.T) {
	t.Parallel()

	dev := &fakeDevice{downloadErr: errors.New("download failed")}
	flasher := newFlasherFor(dev, nil)

	activation, err := flasher.Flash(context.Background(), Target{DevPath: "/dev/nvme0"}, []byte("image"))
	require.ErrorIs(t, err, dev.downloadErr)
	assert.Equal(t, ActivationNone, activation)
	assert.Equal(t, 1, dev.closes) // device closed even on failure
	assert.Zero(t, dev.commits)    // commit never reached
}

func TestNvmeFlasherFlashCommitError(t *testing.T) {
	t.Parallel()

	// a device that reports an activation alongside an error must not leak it
	dev := &fakeDevice{commitErr: errors.New("commit failed")}
	flasher := newFlasherFor(dev, nil)

	activation, err := flasher.Flash(context.Background(), Target{DevPath: "/dev/nvme0"}, []byte("image"))
	require.ErrorIs(t, err, dev.commitErr)
	assert.Equal(t, ActivationNone, activation)
	assert.Equal(t, 1, dev.commits)
	assert.Equal(t, 1, dev.closes)
}

func TestNvmeFlasherFlashOpenError(t *testing.T) {
	t.Parallel()

	openErr := errors.New("no such device")
	dev := &fakeDevice{}
	flasher := newFlasherFor(dev, openErr)

	activation, err := flasher.Flash(context.Background(), Target{DevPath: "/dev/nvme0"}, []byte("image"))
	require.ErrorIs(t, err, openErr)
	assert.Equal(t, ActivationNone, activation)
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

	activation, err := flasher.Flash(ctx, Target{DevPath: "/dev/nvme0"}, []byte("image"))
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, ActivationNone, activation)
	assert.Empty(t, dev.downloads)
	assert.Zero(t, dev.commits)
	assert.Zero(t, dev.closes)
}

// blockingDevice is an nvmeDevice whose download records how many downloads
// of its subsystem overlap.
type blockingDevice struct {
	subsystem string
	inFlight  *atomic.Int32
	maxSeen   *atomic.Int32
}

func (b blockingDevice) Subsystem() string { return b.subsystem }

func (b blockingDevice) DownloadFirmware(context.Context, []byte) error {
	n := b.inFlight.Add(1)
	defer b.inFlight.Add(-1)

	for {
		seen := b.maxSeen.Load()
		if n <= seen || b.maxSeen.CompareAndSwap(seen, n) {
			break
		}
	}

	time.Sleep(10 * time.Millisecond)

	return nil
}

func (blockingDevice) Commit(context.Context) (Activation, error) { return ActivationReboot, nil }

func (blockingDevice) Close() error { return nil }

func TestNvmeFlasherSerializesSubsystem(t *testing.T) {
	t.Parallel()

	// e.g. /dev/nvme0n1 and /dev/nvme0n2, or two paths of one multipath
	// subsystem: their flashes must not overlap.
	var inFlight, maxSeen atomic.Int32

	subsystem := t.Name()
	flasher := nvmeFlasher{open: func(string) (nvmeDevice, error) {
		return blockingDevice{subsystem: subsystem, inFlight: &inFlight, maxSeen: &maxSeen}, nil
	}}

	var wg sync.WaitGroup

	for range 4 {
		wg.Go(func() {
			activation, err := flasher.Flash(context.Background(), Target{DevPath: "/dev/nvme0n1"}, []byte("image"))
			assert.NoError(t, err)
			assert.Equal(t, ActivationReboot, activation)
		})
	}

	wg.Wait()

	assert.Equal(t, int32(1), maxSeen.Load())
}

func TestNvmeFlasherCanceledWaitingForSubsystem(t *testing.T) {
	t.Parallel()

	dev := &fakeDevice{subsystem: t.Name()}
	flasher := newFlasherFor(dev, nil)

	unlock, err := lockSubsystem(context.Background(), t.Name())
	require.NoError(t, err)

	defer unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	activation, err := flasher.Flash(ctx, Target{DevPath: "/dev/nvme0n2"}, []byte("image"))
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.ErrorContains(t, err, "waiting for another flash of "+t.Name())
	assert.Equal(t, ActivationNone, activation)
	assert.Empty(t, dev.downloads)
	assert.Equal(t, 1, dev.closes)
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
	t.Cleanup(func() { delete(registry, "test.flash.example") })

	f, ok := For("test.flash.example")
	require.True(t, ok)

	_, err := f.Flash(context.Background(), Target{DevPath: "/dev/x"}, []byte("fw"))
	require.NoError(t, err)
	require.Len(t, dev.downloads, 1)
	assert.Equal(t, 1, dev.commits)
}

func TestActivationOrder(t *testing.T) {
	t.Parallel()

	// callers combine activations across devices with max(), so "nothing
	// staged" must sort below every real activation, in order of disruption.
	assert.Less(t, ActivationNone, ActivationReboot)
	assert.Less(t, ActivationReboot, ActivationFullReboot)
	assert.Less(t, ActivationFullReboot, ActivationPowerCycle)
	assert.Zero(t, ActivationNone)
}

func TestNvmeFlasherFlashNilOpenFallback(t *testing.T) {
	t.Parallel()

	// nvmeFlasher{} (zero-value, nil open) should fall back to openNVMe rather
	// than panic with a nil-function call. We can't exercise the real openNVMe
	// without a device, so we assert the fallback by observing that it tries
	// to open the Target.DevPath — a nonexistent path surfaces as an error,
	// not a panic.
	_, err := nvmeFlasher{}.Flash(context.Background(), Target{DevPath: "/nonexistent-nvme-device"}, []byte("image"))
	require.Error(t, err)
	require.ErrorContains(t, err, "/nonexistent-nvme-device")
}
