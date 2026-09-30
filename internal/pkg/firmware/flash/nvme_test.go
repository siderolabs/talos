// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage // tests validate unexported struct layout against the kernel ABI.
package flash

import (
	"context"
	"fmt"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestAdminCmdLayout(t *testing.T) {
	t.Parallel()

	// must match struct nvme_admin_cmd from <linux/nvme_ioctl.h>
	assert.Equal(t, uintptr(72), unsafe.Sizeof(adminCmd{}))
	assert.Equal(t, uintptr(4), unsafe.Offsetof(adminCmd{}.nsid))
	assert.Equal(t, uintptr(16), unsafe.Offsetof(adminCmd{}.metadata))
	assert.Equal(t, uintptr(24), unsafe.Offsetof(adminCmd{}.addr))
	assert.Equal(t, uintptr(36), unsafe.Offsetof(adminCmd{}.dataLen))
	assert.Equal(t, uintptr(40), unsafe.Offsetof(adminCmd{}.cdw10))
	assert.Equal(t, uintptr(64), unsafe.Offsetof(adminCmd{}.timeoutMS))
	assert.Equal(t, uintptr(68), unsafe.Offsetof(adminCmd{}.result))
}

func TestIoctlNumber(t *testing.T) {
	t.Parallel()

	// _IOWR('N', 0x41, struct nvme_admin_cmd)
	expected := uint32(3<<30) | uint32(unsafe.Sizeof(adminCmd{}))<<16 | uint32('N')<<8 | 0x41

	assert.Equal(t, expected, uint32(nvmeIoctlAdminCmd))
}

// ioctlRecorder captures commands via the NVMeDevice.ioctl seam and returns
// canned results. failOn, when nonzero, fails the n-th command with EIO.
// fwug and mdts are written into the Identify Controller response buffer.
type ioctlRecorder struct {
	commands []adminCmd
	status   uintptr
	errno    unix.Errno
	failOn   int

	fwug uint8
	mdts uint8
}

func (r *ioctlRecorder) ioctl(_ int, cmd *adminCmd, buf []byte) (uintptr, unix.Errno) {
	r.commands = append(r.commands, *cmd)

	if r.failOn == len(r.commands) {
		return 0, unix.EIO
	}

	if cmd.opcode == opcodeIdentify {
		buf[identifyMDTSOffset] = r.mdts
		buf[identifyFWUGOffset] = r.fwug
	}

	return r.status, r.errno
}

// preIdentified returns a device that skips the Identify call with FWUG=1
// (default 4 KiB chunks) and no MDTS cap. Used by tests that don't exercise
// the Identify path.
func preIdentified(rec *ioctlRecorder) *NVMeDevice {
	return &NVMeDevice{ioctl: rec.ioctl, fwug: 1, identified: true}
}

func TestDownloadFirmwareChunking(t *testing.T) {
	t.Parallel()

	image := make([]byte, 2*defaultDownloadChunkSize+101)
	for i := range image {
		image[i] = byte(i%251 + 1)
	}

	rec := &ioctlRecorder{}
	dev := preIdentified(rec)

	require.NoError(t, dev.DownloadFirmware(context.Background(), image))
	// 3 download chunks (the 101-byte tail padded to a full 4 KiB chunk).
	require.Len(t, rec.commands, 3)

	for i, cmd := range rec.commands {
		assert.Equal(t, uint8(opcodeFirmwareDownload), cmd.opcode, "chunk %d opcode", i)
		assert.NotZero(t, cmd.addr, "chunk %d data address", i)
		assert.NotZero(t, cmd.timeoutMS, "chunk %d timeout", i)
		assert.Equal(t, uint32(defaultDownloadChunkSize), cmd.dataLen, "chunk %d dataLen", i)
		assert.Equal(t, uint32(defaultDownloadChunkSize>>2)-1, cmd.cdw10, "chunk %d NUMD", i)
	}

	// offsets advance in dwords
	assert.Equal(t, uint32(0), rec.commands[0].cdw11)
	assert.Equal(t, uint32(defaultDownloadChunkSize>>2), rec.commands[1].cdw11)
	assert.Equal(t, uint32(2*defaultDownloadChunkSize>>2), rec.commands[2].cdw11)
}

func TestFirmwareChunk(t *testing.T) {
	t.Parallel()

	image := make([]byte, 2*defaultDownloadChunkSize+101)
	for i := range image {
		image[i] = byte(i%251 + 1)
	}

	// full chunks pass through as views of image
	assert.Equal(t, image[:defaultDownloadChunkSize], firmwareChunk(image, 0, defaultDownloadChunkSize))
	assert.Equal(t, image[defaultDownloadChunkSize:2*defaultDownloadChunkSize], firmwareChunk(image, defaultDownloadChunkSize, defaultDownloadChunkSize))

	// final chunk is padded to the full chunkSize with 0xff
	tail := firmwareChunk(image, 2*defaultDownloadChunkSize, defaultDownloadChunkSize)
	require.Len(t, tail, defaultDownloadChunkSize)
	assert.Equal(t, image[2*defaultDownloadChunkSize:], tail[:101])

	for i := 101; i < defaultDownloadChunkSize; i++ {
		assert.Equal(t, byte(0xff), tail[i], "pad byte %d", i)
	}
}

func TestFirmwareChunkSmall(t *testing.T) {
	t.Parallel()

	// a sub-chunk image is padded to the full chunkSize
	tail := firmwareChunk([]byte{1, 2, 3}, 0, defaultDownloadChunkSize)
	require.Len(t, tail, defaultDownloadChunkSize)
	assert.Equal(t, []byte{1, 2, 3}, tail[:3])

	for i := 3; i < defaultDownloadChunkSize; i++ {
		assert.Equal(t, byte(0xff), tail[i], "pad byte %d", i)
	}
}

func TestDownloadFirmwareSmallImage(t *testing.T) {
	t.Parallel()

	// sub-chunk image; unpadded cdw10 would underflow to 0xffffffff.
	rec := &ioctlRecorder{}
	dev := preIdentified(rec)

	require.NoError(t, dev.DownloadFirmware(context.Background(), []byte{1, 2, 3}))
	require.Len(t, rec.commands, 1)

	assert.Equal(t, uint32(defaultDownloadChunkSize), rec.commands[0].dataLen)
	assert.Equal(t, uint32(defaultDownloadChunkSize>>2)-1, rec.commands[0].cdw10)
	assert.Equal(t, uint32(0), rec.commands[0].cdw11)
}

func TestDownloadFirmwareEmpty(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{}
	dev := preIdentified(rec)

	require.Error(t, dev.DownloadFirmware(context.Background(), nil))
	assert.Empty(t, rec.commands)
}

func TestDownloadFirmwareError(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{failOn: 2}
	dev := preIdentified(rec)

	err := dev.DownloadFirmware(context.Background(), make([]byte, 2*defaultDownloadChunkSize))
	require.ErrorIs(t, err, unix.EIO)
	require.ErrorContains(t, err, fmt.Sprintf("offset %d", defaultDownloadChunkSize))
	require.Len(t, rec.commands, 2) // failed on the second chunk
}

func TestDownloadFirmwareCanceledBeforeStart(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{}
	dev := preIdentified(rec)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.ErrorIs(t, dev.DownloadFirmware(ctx, make([]byte, 1)), context.Canceled)
	assert.Empty(t, rec.commands)
}

func TestDownloadFirmwareCanceledMidway(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	rec := &ioctlRecorder{}
	dev := &NVMeDevice{
		fwug:       1,
		identified: true,
		ioctl: func(fd int, cmd *adminCmd, buf []byte) (uintptr, unix.Errno) {
			cancel() // cancel while the first chunk is in flight

			return rec.ioctl(fd, cmd, buf)
		},
	}

	err := dev.DownloadFirmware(ctx, make([]byte, 2*defaultDownloadChunkSize))
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorContains(t, err, fmt.Sprintf("offset %d", defaultDownloadChunkSize))
	require.Len(t, rec.commands, 1) // the second chunk is never submitted
}

func TestCommit(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{}
	dev := &NVMeDevice{ioctl: rec.ioctl}

	activation, err := dev.Commit(context.Background())
	require.NoError(t, err)
	assert.Equal(t, ActivationReboot, activation)
	require.Len(t, rec.commands, 1)

	cmd := rec.commands[0]
	assert.Equal(t, uint8(opcodeFirmwareCommit), cmd.opcode)
	assert.Equal(t, uint32(0b001<<3), cmd.cdw10) // CA=0b001, FS=0 (controller-chosen slot)
	assert.Zero(t, cmd.addr)
	assert.Zero(t, cmd.dataLen)
	assert.NotZero(t, cmd.timeoutMS)
}

func TestCommitCanceled(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{}
	dev := &NVMeDevice{ioctl: rec.ioctl}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := dev.Commit(ctx)
	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, rec.commands)
}

func TestCommitStatuses(t *testing.T) {
	t.Parallel()

	// Reset-required statuses are success for Firmware Commit and map to the
	// activation the host must perform; any other non-zero NVMe status is a
	// real failure.
	for _, tc := range []struct {
		name    string
		status  uintptr
		errno   unix.Errno
		want    Activation
		wantErr string
	}{
		{name: "success", status: statusSuccess, want: ActivationReboot},
		{name: "fw needs conventional reset", status: statusFWNeedsConvReset, want: ActivationFullReboot},
		{name: "fw needs subsystem reset", status: statusFWNeedsSubsysReset, want: ActivationPowerCycle},
		{name: "fw needs reset", status: statusFWNeedsReset, want: ActivationReboot},
		{name: "fw image error", status: 0x107, wantErr: "0x107"},
		{name: "SCT=4 would collapse under 0x3ff mask", status: 0x400, wantErr: "0x400"},
		{name: "SCT=5 would collapse under 0x3ff mask", status: 0x50b, wantErr: "0x50b"},
		{name: "errno failure", errno: unix.EIO, wantErr: "input/output"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := &ioctlRecorder{status: tc.status, errno: tc.errno}
			dev := &NVMeDevice{ioctl: rec.ioctl}

			activation, err := dev.Commit(context.Background())

			if tc.wantErr == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.want, activation)

				return
			}

			require.Error(t, err)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestAdminPassthruRejectsResetStatuses(t *testing.T) {
	t.Parallel()

	// Reset-required statuses must NOT be swallowed for non-Commit opcodes:
	// if adminPassthru returned nil for Identify when the controller reported
	// e.g. 0x10b, callers would read zeroed data and silently misbehave.
	for _, tc := range []struct {
		name   string
		status uintptr
	}{
		{name: "0x10b conv reset", status: statusFWNeedsConvReset},
		{name: "0x110 subsys reset", status: statusFWNeedsSubsysReset},
		{name: "0x111 reset", status: statusFWNeedsReset},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := &ioctlRecorder{status: tc.status}
			dev := &NVMeDevice{ioctl: rec.ioctl}

			_, err := dev.chunkSize()
			require.Error(t, err)
			require.ErrorContains(t, err, "identify controller")
		})
	}
}

func TestChunkSize(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		fwug    uint8
		mdts    uint8
		wantLen int
	}{
		{name: "no info (FWUG=0, MDTS=0)", fwug: 0, mdts: 0, wantLen: defaultDownloadChunkSize},
		{name: "FWUG=1", fwug: 1, mdts: 0, wantLen: defaultDownloadChunkSize},
		{name: "FWUG=2", fwug: 2, mdts: 0, wantLen: 2 * defaultDownloadChunkSize},
		{name: "no restriction (FWUG=0xff)", fwug: 0xff, mdts: 0, wantLen: defaultDownloadChunkSize},
		{name: "MDTS equal to FWUG", fwug: 4, mdts: 4, wantLen: 4 * defaultDownloadChunkSize},
		{name: "MDTS above FWUG does not change size", fwug: 2, mdts: 5, wantLen: 2 * defaultDownloadChunkSize},
		{name: "MDTS=0 means unbounded", fwug: 0x80, mdts: 0, wantLen: 0x80 * defaultDownloadChunkSize},
		{name: "pathological MDTS exponent is ignored", fwug: 1, mdts: 0xff, wantLen: defaultDownloadChunkSize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := &ioctlRecorder{fwug: tc.fwug, mdts: tc.mdts}
			dev := &NVMeDevice{ioctl: rec.ioctl}

			got, err := dev.chunkSize()
			require.NoError(t, err)
			assert.Equal(t, tc.wantLen, got)

			// caches after first Identify.
			_, err = dev.chunkSize()
			require.NoError(t, err)
			require.Len(t, rec.commands, 1)
			assert.Equal(t, uint8(opcodeIdentify), rec.commands[0].opcode)
			assert.Equal(t, uint32(cnsIdentifyController), rec.commands[0].cdw10)
			assert.Equal(t, uint32(identifyControllerDataLen), rec.commands[0].dataLen)
		})
	}
}

func TestChunkSizeIdentifyError(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{failOn: 1}
	dev := &NVMeDevice{ioctl: rec.ioctl}

	_, err := dev.chunkSize()
	require.ErrorIs(t, err, unix.EIO)
	require.ErrorContains(t, err, "identify controller")
}

func TestChunkSizeFWUGExceedsMDTS(t *testing.T) {
	t.Parallel()

	// FWUG=4 (16 KiB required) with MDTS=1 (8 KiB max transfer) — no valid
	// chunk size satisfies both; chunkSize must surface an error rather than
	// quietly producing a non-FWUG-aligned chunk the controller would reject.
	rec := &ioctlRecorder{fwug: 4, mdts: 1}
	dev := &NVMeDevice{ioctl: rec.ioctl}

	_, err := dev.chunkSize()
	require.Error(t, err)
	require.ErrorContains(t, err, "exceeds MDTS limit")
}

func TestDownloadFirmwareLargerChunkFromFWUG(t *testing.T) {
	t.Parallel()

	// FWUG=2 means the controller requires 8 KiB chunks and the final chunk
	// must also be a multiple of FWUG * 4 KiB — a 12 KiB image produces two
	// 8 KiB downloads (the second half-filled with 0xff), plus Identify.
	rec := &ioctlRecorder{fwug: 2}
	dev := &NVMeDevice{ioctl: rec.ioctl}

	require.NoError(t, dev.DownloadFirmware(context.Background(), make([]byte, 3*defaultDownloadChunkSize)))
	require.Len(t, rec.commands, 3)

	assert.Equal(t, uint8(opcodeIdentify), rec.commands[0].opcode)

	for i, cmd := range rec.commands[1:] {
		assert.Equal(t, uint8(opcodeFirmwareDownload), cmd.opcode, "download %d opcode", i)
		assert.Equal(t, uint32(2*defaultDownloadChunkSize), cmd.dataLen, "download %d dataLen (should be padded to 8 KiB)", i)
	}
}

func TestOpenNVMeMissing(t *testing.T) {
	t.Parallel()

	_, err := OpenNVMe("/nonexistent-nvme-device")
	require.Error(t, err)
	require.ErrorContains(t, err, "/nonexistent-nvme-device")
}

func TestOpenNVMeCloexec(t *testing.T) {
	t.Parallel()

	dev, err := OpenNVMe("/dev/null")
	require.NoError(t, err)

	defer dev.Close() //nolint:errcheck

	flags, err := unix.FcntlInt(uintptr(dev.fd), unix.F_GETFD, 0)
	require.NoError(t, err)
	assert.NotZero(t, flags&unix.FD_CLOEXEC, "device fd must be close-on-exec")
}
