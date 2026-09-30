// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux && (amd64 || arm64 || loong64 || riscv64 || s390x)

//nolint:testpackage // tests validate unexported struct layout against the kernel ABI.
package flash

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	assert.Equal(t, uintptr(8), unsafe.Sizeof(adminCmd{}.addr)) // __u64 addr
	assert.Equal(t, uintptr(36), unsafe.Offsetof(adminCmd{}.dataLen))
	assert.Equal(t, uintptr(40), unsafe.Offsetof(adminCmd{}.cdw10))
	assert.Equal(t, uintptr(64), unsafe.Offsetof(adminCmd{}.timeoutMS))
	assert.Equal(t, uintptr(68), unsafe.Offsetof(adminCmd{}.result))
}

func TestIoctlNumbers(t *testing.T) {
	t.Parallel()

	// _IOWR('N', 0x41, struct nvme_admin_cmd)
	adminCmdReq := uint32(3<<30) | uint32(unsafe.Sizeof(adminCmd{}))<<16 | uint32('N')<<8 | 0x41
	assert.Equal(t, adminCmdReq, uint32(nvmeIoctlAdminCmd))

	// _IO('N', 0x44)
	assert.Equal(t, uint32('N')<<8|0x44, uint32(nvmeIoctlReset))
}

// ioctlRecorder captures commands via the NVMeDevice.ioctl seam and returns
// canned results. failOn, when nonzero, fails the n-th admin command with EIO.
// statusSeq gives per-command statuses for the first len(statusSeq) admin
// commands; later ones return status. fwug and mdts are written into the
// Identify Controller response buffer. Resets are counted, not recorded.
type ioctlRecorder struct {
	commands  []adminCmd
	status    uintptr
	statusSeq []uintptr
	errno     unix.Errno
	failOn    int

	resets     int
	resetErrno unix.Errno

	fwug uint8
	mdts uint8
}

func (r *ioctlRecorder) ioctl(_ int, req uint, cmd *adminCmd, buf []byte) (uintptr, unix.Errno) {
	if req == nvmeIoctlReset {
		r.resets++

		return 0, r.resetErrno
	}

	r.commands = append(r.commands, *cmd)

	n := len(r.commands)

	if r.failOn == n {
		return 0, unix.EIO
	}

	if cmd.opcode == opcodeIdentify {
		buf[identifyMDTSOffset] = r.mdts
		buf[identifyFWUGOffset] = r.fwug
	}

	if n <= len(r.statusSeq) {
		return r.statusSeq[n-1], r.errno
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
	assert.Zero(t, rec.resets)

	for i, cmd := range rec.commands {
		assert.Equal(t, uint8(opcodeFirmwareDownload), cmd.opcode, "chunk %d opcode", i)
		assert.NotNil(t, cmd.addr, "chunk %d data address", i)
		assert.NotZero(t, cmd.timeoutMS, "chunk %d timeout", i)
		assert.Equal(t, uint32(defaultDownloadChunkSize), cmd.dataLen, "chunk %d dataLen", i)
		assert.Equal(t, uint32(defaultDownloadChunkSize>>2)-1, cmd.cdw10, "chunk %d NUMD", i)
	}

	// the first two chunks point into the image itself
	assert.Equal(t, unsafe.Pointer(&image[0]), rec.commands[0].addr)
	assert.Equal(t, unsafe.Pointer(&image[defaultDownloadChunkSize]), rec.commands[1].addr)

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

func TestDownloadFirmwareErrorResetsPartialImage(t *testing.T) {
	t.Parallel()

	// the first chunk was accepted, so the controller holds a partial image:
	// it must be reset before returning the error.
	rec := &ioctlRecorder{failOn: 2}
	dev := preIdentified(rec)

	err := dev.DownloadFirmware(context.Background(), make([]byte, 2*defaultDownloadChunkSize))
	require.ErrorIs(t, err, unix.EIO)
	require.ErrorContains(t, err, fmt.Sprintf("offset %d", defaultDownloadChunkSize))
	require.Len(t, rec.commands, 2) // failed on the second chunk, no retry
	assert.Equal(t, 1, rec.resets)
}

func TestDownloadFirmwareFirstChunkErrorNoReset(t *testing.T) {
	t.Parallel()

	// nothing was accepted, so there is nothing to discard.
	rec := &ioctlRecorder{failOn: 1}
	dev := preIdentified(rec)

	err := dev.DownloadFirmware(context.Background(), make([]byte, 2*defaultDownloadChunkSize))
	require.ErrorIs(t, err, unix.EIO)
	require.Len(t, rec.commands, 1)
	assert.Zero(t, rec.resets)
}

func TestDownloadFirmwarePartialImageResetFails(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{failOn: 2, resetErrno: unix.EBUSY}
	dev := preIdentified(rec)

	err := dev.DownloadFirmware(context.Background(), make([]byte, 2*defaultDownloadChunkSize))
	require.ErrorIs(t, err, unix.EIO)
	require.ErrorContains(t, err, "partial firmware image")
	require.ErrorContains(t, err, "controller reset failed: device or resource busy")
	require.ErrorContains(t, err, "reboot before retrying")
	assert.Equal(t, 1, rec.resets)
}

func TestDownloadFirmwareStaleImageIsDiscarded(t *testing.T) {
	t.Parallel()

	// 0x114 on the very first chunk means an earlier download was never
	// committed: reset the controller and start over, once.
	rec := &ioctlRecorder{statusSeq: []uintptr{statusOverlappingRange, statusSuccess, statusSuccess}}
	dev := preIdentified(rec)

	require.NoError(t, dev.DownloadFirmware(context.Background(), make([]byte, 2*defaultDownloadChunkSize)))
	require.Len(t, rec.commands, 3) // rejected chunk 0, then chunks 0 and 1
	assert.Equal(t, 1, rec.resets)
	assert.Equal(t, uint32(0), rec.commands[1].cdw11, "retry restarts at offset 0")
	assert.Equal(t, uint32(defaultDownloadChunkSize>>2), rec.commands[2].cdw11)
}

func TestDownloadFirmwareStaleImageRetriedOnce(t *testing.T) {
	t.Parallel()

	// still 0x114 after a reset: give up rather than loop.
	rec := &ioctlRecorder{status: statusOverlappingRange}
	dev := preIdentified(rec)

	err := dev.DownloadFirmware(context.Background(), make([]byte, 2*defaultDownloadChunkSize))
	require.ErrorContains(t, err, "0x114 (overlapping range)")
	require.Len(t, rec.commands, 2)
	assert.Equal(t, 1, rec.resets)
}

func TestDownloadFirmwareStaleImageResetFails(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{status: statusOverlappingRange, resetErrno: unix.EBUSY}
	dev := preIdentified(rec)

	err := dev.DownloadFirmware(context.Background(), make([]byte, 2*defaultDownloadChunkSize))
	require.ErrorContains(t, err, "0x114 (overlapping range)")
	require.ErrorContains(t, err, "stale firmware image")
	require.ErrorContains(t, err, "reboot before retrying")
	require.Len(t, rec.commands, 1) // no retry without a reset
	assert.Equal(t, 1, rec.resets)
}

func TestDownloadFirmwareOverlapAfterProgressIsNotRetried(t *testing.T) {
	t.Parallel()

	// 0x114 after chunks were accepted is a real failure of this download, not
	// a stale image: reset for cleanup, but do not retry.
	rec := &ioctlRecorder{statusSeq: []uintptr{statusSuccess, statusOverlappingRange}}
	dev := preIdentified(rec)

	err := dev.DownloadFirmware(context.Background(), make([]byte, 2*defaultDownloadChunkSize))
	require.ErrorContains(t, err, fmt.Sprintf("offset %d", defaultDownloadChunkSize))
	require.Len(t, rec.commands, 2)
	assert.Equal(t, 1, rec.resets)
}

func TestDownloadFirmwareCanceledBeforeStart(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{}
	dev := preIdentified(rec)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.ErrorIs(t, dev.DownloadFirmware(ctx, make([]byte, 1)), context.Canceled)
	assert.Empty(t, rec.commands)
	assert.Zero(t, rec.resets)
}

func TestDownloadFirmwareCanceledMidway(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	rec := &ioctlRecorder{}
	dev := &NVMeDevice{
		fwug:       1,
		identified: true,
		ioctl: func(fd int, req uint, cmd *adminCmd, buf []byte) (uintptr, unix.Errno) {
			cancel() // cancel while the first chunk is in flight

			return rec.ioctl(fd, req, cmd, buf)
		},
	}

	err := dev.DownloadFirmware(ctx, make([]byte, 2*defaultDownloadChunkSize))
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorContains(t, err, fmt.Sprintf("offset %d", defaultDownloadChunkSize))
	require.Len(t, rec.commands, 1) // the second chunk is never submitted
	assert.Equal(t, 1, rec.resets)  // the accepted chunk is discarded
}

func TestReset(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{}
	dev := &NVMeDevice{ioctl: rec.ioctl}

	require.NoError(t, dev.Reset())
	assert.Equal(t, 1, rec.resets)
	assert.Empty(t, rec.commands)

	rec.resetErrno = unix.ENOTTY

	require.ErrorIs(t, dev.Reset(), unix.ENOTTY)
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
	assert.Nil(t, cmd.addr)
	assert.Zero(t, cmd.dataLen)
	assert.NotZero(t, cmd.timeoutMS)
}

func TestCommitCanceled(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{}
	dev := &NVMeDevice{ioctl: rec.ioctl}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// canceled after a complete download: the image must not stay staged.
	activation, err := dev.Commit(ctx)
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, ActivationNone, activation)
	assert.Empty(t, rec.commands)
	assert.Equal(t, 1, rec.resets)
}

func TestCommitRejectedResetFails(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{status: 0x107, resetErrno: unix.EBUSY}
	dev := &NVMeDevice{ioctl: rec.ioctl}

	activation, err := dev.Commit(context.Background())
	require.ErrorContains(t, err, "0x107 (invalid firmware image)")
	require.ErrorContains(t, err, "uncommitted firmware image")
	require.ErrorContains(t, err, "controller reset failed: device or resource busy")
	require.ErrorContains(t, err, "reboot before retrying")
	assert.Equal(t, ActivationNone, activation)
	assert.Equal(t, 1, rec.resets)
}

func TestCommitStatuses(t *testing.T) {
	t.Parallel()

	// Reset-required statuses are success for Firmware Commit and map to the
	// activation the host must perform; any other non-zero NVMe status is a
	// real failure, reported with ActivationNone after discarding the image.
	// An errno failure leaves the outcome unknown, so nothing is reset.
	for _, tc := range []struct {
		name       string
		status     uintptr
		errno      unix.Errno
		want       Activation
		wantErr    string
		wantResets int
	}{
		{name: "success", status: statusSuccess, want: ActivationReboot},
		{name: "fw needs conventional reset", status: statusFWNeedsConvReset, want: ActivationFullReboot},
		{name: "fw needs subsystem reset", status: statusFWNeedsSubsysReset, want: ActivationPowerCycle},
		{name: "fw needs reset", status: statusFWNeedsReset, want: ActivationReboot},
		{name: "fw image error", status: 0x107, wantErr: "0x107 (invalid firmware image)", wantResets: 1},
		{name: "overlapping range left by an uncommitted download", status: 0x114, wantErr: "0x114 (overlapping range)", wantResets: 1},
		{name: "unlisted status", status: 0x0ff, wantErr: "status 0x0ff", wantResets: 1},
		{name: "SCT=4 would collapse under 0x3ff mask", status: 0x400, wantErr: "0x400", wantResets: 1},
		{name: "SCT=5 would collapse under 0x3ff mask", status: 0x50b, wantErr: "0x50b", wantResets: 1},
		{name: "errno failure", errno: unix.EIO, wantErr: "input/output error; whether the firmware was committed is unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := &ioctlRecorder{status: tc.status, errno: tc.errno}
			dev := &NVMeDevice{ioctl: rec.ioctl}

			activation, err := dev.Commit(context.Background())

			assert.Equal(t, tc.wantResets, rec.resets)

			if tc.wantErr == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.want, activation)

				return
			}

			require.Error(t, err)
			require.ErrorContains(t, err, tc.wantErr)
			assert.Equal(t, ActivationNone, activation)
		})
	}
}

func TestAdminPassthruStatusNames(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		status uintptr
		want   string
	}{
		{status: 0x114, want: "admin command 0x10 failed with NVMe status 0x114 (overlapping range)"},
		{status: 0x0ff, want: "admin command 0x10 failed with NVMe status 0x0ff"},
	} {
		rec := &ioctlRecorder{status: tc.status}
		dev := &NVMeDevice{ioctl: rec.ioctl}

		_, err := dev.Commit(context.Background())
		require.EqualError(t, err, tc.want)
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
		name        string
		fwug        uint8
		mdts        uint8
		maxTransfer int
		wantLen     int
	}{
		{name: "no info (FWUG=0, MDTS=0)", fwug: 0, mdts: 0, wantLen: defaultDownloadChunkSize},
		{name: "FWUG=1", fwug: 1, mdts: 0, wantLen: defaultDownloadChunkSize},
		{name: "FWUG=2", fwug: 2, mdts: 0, wantLen: 2 * defaultDownloadChunkSize},
		{name: "no restriction (FWUG=0xff)", fwug: 0xff, mdts: 0, wantLen: defaultDownloadChunkSize},
		{name: "MDTS equal to FWUG", fwug: 4, mdts: 4, wantLen: 4 * defaultDownloadChunkSize},
		{name: "MDTS above FWUG does not change size", fwug: 2, mdts: 5, wantLen: 2 * defaultDownloadChunkSize},
		{name: "MDTS=0 means unbounded", fwug: 0x80, mdts: 0, wantLen: 0x80 * defaultDownloadChunkSize},
		{name: "pathological MDTS exponent is ignored", fwug: 1, mdts: 0xff, wantLen: defaultDownloadChunkSize},
		{name: "kernel limit equal to FWUG", fwug: 0x20, mdts: 0, maxTransfer: 128 * 1024, wantLen: 0x20 * defaultDownloadChunkSize},
		{name: "kernel limit above FWUG does not change size", fwug: 2, mdts: 0, maxTransfer: 128 * 1024, wantLen: 2 * defaultDownloadChunkSize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := &ioctlRecorder{fwug: tc.fwug, mdts: tc.mdts}
			dev := &NVMeDevice{ioctl: rec.ioctl, maxTransfer: tc.maxTransfer}

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

func TestChunkSizeFWUGExceedsKernelLimit(t *testing.T) {
	t.Parallel()

	// FWUG=0x80 (512 KiB) with MDTS=0 passes the MDTS check, but the kernel
	// caps passthrough buffers at max_hw_sectors_kb (here 128 KiB) and would
	// reject every chunk with EINVAL; name the limit instead.
	rec := &ioctlRecorder{fwug: 0x80, mdts: 0}
	dev := &NVMeDevice{ioctl: rec.ioctl, maxTransfer: 128 * 1024}

	_, err := dev.chunkSize()
	require.Error(t, err)
	require.ErrorContains(t, err, "exceeds the kernel transfer limit of 131072 bytes")
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

func TestDownloadFirmwareEINVALHint(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		maxTransfer int
		failOn      int
		wantHint    bool
	}{
		// the kernel limit is unknown and the first chunk is rejected: the
		// chunk size is the likely cause.
		{name: "unknown limit, first chunk", failOn: 1, wantHint: true},
		// a known limit was already checked by chunkSize.
		{name: "known limit", maxTransfer: 128 * 1024, failOn: 1},
		// an equally sized chunk was already accepted.
		{name: "unknown limit, later chunk", failOn: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			n := 0
			rec := &ioctlRecorder{}
			dev := &NVMeDevice{
				fwug:        1,
				identified:  true,
				maxTransfer: tc.maxTransfer,
				ioctl: func(fd int, req uint, cmd *adminCmd, buf []byte) (uintptr, unix.Errno) {
					if req == nvmeIoctlAdminCmd {
						n++

						if n == tc.failOn {
							return 0, unix.EINVAL
						}
					}

					return rec.ioctl(fd, req, cmd, buf)
				},
			}

			err := dev.DownloadFirmware(context.Background(), make([]byte, 2*defaultDownloadChunkSize))
			require.ErrorIs(t, err, unix.EINVAL)

			if tc.wantHint {
				require.ErrorContains(t, err, "the 4096-byte chunk may exceed the kernel transfer limit")
			} else {
				require.NotContains(t, err.Error(), "kernel transfer limit")
			}
		})
	}
}

// fakeSysfs builds the parts of sysfs that controllerForNode and
// kernelMaxTransfer read, modeled on a real system:
//
//   - nvme0: a single-port PCIe controller in subsystem nvme-subsys0, with
//     namespace nvme0n1 (partition nvme0n1p1), max_hw_sectors_kb 128;
//   - nvme-subsys1: a multipath subsystem with controllers nvme1 (connecting,
//     path device nvme1c1n1, 64 KiB) and nvme2 (live, nvme2c2n1, 256 KiB),
//     exposing the shared namespace nvme1n1;
//   - sda: a SCSI disk, and /dev/null: a non-NVMe character device.
func fakeSysfs(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	mkdir := func(p string) {
		require.NoError(t, os.MkdirAll(filepath.Join(root, p), 0o755))
	}

	write := func(p, content string) {
		mkdir(filepath.Dir(p))
		require.NoError(t, os.WriteFile(filepath.Join(root, p), []byte(content+"\n"), 0o644))
	}

	link := func(p, target string) {
		mkdir(filepath.Dir(p))
		require.NoError(t, os.Symlink(target, filepath.Join(root, p)))
	}

	const (
		nvme0 = "devices/pci0000:00/0000:02:00.0/nvme/nvme0"
		nvme1 = "devices/pci0000:00/0000:03:00.0/nvme/nvme1"
		nvme2 = "devices/pci0000:00/0000:03:00.1/nvme/nvme2"
		sub0  = "devices/virtual/nvme-subsystem/nvme-subsys0"
		sub1  = "devices/virtual/nvme-subsystem/nvme-subsys1"
		sda   = "devices/pci0000:00/0000:01:00.0/host0/target0:0:0/0:0:0:0/block/sda"
	)

	write(nvme0+"/state", "live")
	write(nvme0+"/dev", "246:0")
	link(nvme0+"/nvme0n1/device", "../../nvme0")
	write(nvme0+"/nvme0n1/queue/max_hw_sectors_kb", "128")
	write(nvme0+"/nvme0n1/nvme0n1p1/partition", "1")

	write(nvme1+"/state", "connecting")
	write(nvme1+"/nvme1c1n1/queue/max_hw_sectors_kb", "64")
	write(nvme2+"/state", "live")
	write(nvme2+"/nvme2c2n1/queue/max_hw_sectors_kb", "256")
	link(sub1+"/nvme1", "../../../pci0000:00/0000:03:00.0/nvme/nvme1")
	link(sub1+"/nvme2", "../../../pci0000:00/0000:03:00.1/nvme/nvme2")
	link(sub1+"/nvme1n1/device", "../../nvme-subsys1")

	link(sda+"/device", "../../../0:0:0:0")
	mkdir("devices/virtual/mem/null")

	// every controller belongs to a subsystem, multipath or not.
	link(sub0+"/nvme0", "../../../pci0000:00/0000:02:00.0/nvme/nvme0")

	link("class/nvme/nvme0", "../../"+nvme0)
	link("class/nvme/nvme1", "../../"+nvme1)
	link("class/nvme/nvme2", "../../"+nvme2)
	link("class/nvme-subsystem/nvme-subsys0", "../../"+sub0)
	link("class/nvme-subsystem/nvme-subsys1", "../../"+sub1)

	link("dev/block/259:0", "../../"+nvme0+"/nvme0n1")
	link("dev/block/259:1", "../../"+nvme0+"/nvme0n1/nvme0n1p1")
	link("dev/block/259:2", "../../"+sub1+"/nvme1n1")
	link("dev/block/8:0", "../../"+sda)
	link("dev/char/246:0", "../../"+nvme0)
	link("dev/char/1:3", "../../devices/virtual/mem/null")

	return root
}

func TestControllerForNode(t *testing.T) {
	t.Parallel()

	sysfs := fakeSysfs(t)

	for _, tc := range []struct {
		name    string
		char    bool
		node    string
		want    string
		wantErr string
	}{
		{name: "controller character device", char: true, node: "246:0", want: "nvme0"},
		{name: "namespace block device", node: "259:0", want: "nvme0"},
		{name: "partition", node: "259:1", want: "nvme0"},
		{name: "multipath namespace picks the live controller", node: "259:2", want: "nvme2"},
		{name: "SCSI disk", node: "8:0", wantErr: "block device 8:0 is not an NVMe namespace"},
		{name: "non-NVMe character device", char: true, node: "1:3", wantErr: "character device 1:3 is not an NVMe controller"},
		{name: "unknown block device", node: "254:7", wantErr: "error resolving block device 254:7 in sysfs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := controllerForNode(sysfs, tc.char, tc.node)

			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestLiveControllerNoneLive(t *testing.T) {
	t.Parallel()

	// no controller is live: fail rather than send admin commands to a
	// controller that is connecting or resetting.
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "nvme3"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nvme3", "state"), []byte("connecting\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "nvme4"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nvme4", "state"), []byte("resetting\n"), 0o644))

	_, err := liveController(dir)
	require.ErrorContains(t, err, "no live controller")
	require.ErrorContains(t, err, "nvme3, nvme4")

	_, err = liveController(t.TempDir())
	require.ErrorContains(t, err, "no controllers found")
}

func TestSubsystemFor(t *testing.T) {
	t.Parallel()

	sysfs := fakeSysfs(t)

	assert.Equal(t, "nvme-subsys0", subsystemFor(sysfs, "nvme0"))
	assert.Equal(t, "nvme-subsys1", subsystemFor(sysfs, "nvme1"))
	assert.Equal(t, "nvme-subsys1", subsystemFor(sysfs, "nvme2"), "both multipath controllers share one subsystem")
	assert.Equal(t, "nvme9", subsystemFor(sysfs, "nvme9"), "unknown subsystem falls back to the controller")
}

func TestControllerForNonDevice(t *testing.T) {
	t.Parallel()

	_, err := controllerFor(sysfsRoot, "/nonexistent-nvme-device")
	require.ErrorContains(t, err, "/nonexistent-nvme-device")

	regular := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(regular, nil, 0o644))

	_, err = controllerFor(sysfsRoot, regular)
	require.ErrorContains(t, err, "is not a block or character device")
}

func TestKernelMaxTransfer(t *testing.T) {
	t.Parallel()

	sysfs := fakeSysfs(t)

	assert.Equal(t, 128*1024, kernelMaxTransfer(sysfs, "nvme0"))
	assert.Equal(t, 64*1024, kernelMaxTransfer(sysfs, "nvme1"), "multipath path device under the controller")
	assert.Equal(t, 256*1024, kernelMaxTransfer(sysfs, "nvme2"))
	assert.Zero(t, kernelMaxTransfer(sysfs, "nvme9"), "unknown controller")

	// a controller with several namespaces reports the smallest limit, and
	// an unreadable value is skipped.
	ctrl := filepath.Join(sysfs, "class", "nvme", "nvme0")
	require.NoError(t, os.MkdirAll(filepath.Join(ctrl, "nvme0n2", "queue"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ctrl, "nvme0n2", "queue", "max_hw_sectors_kb"), []byte("32\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(ctrl, "nvme0n3", "queue"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ctrl, "nvme0n3", "queue", "max_hw_sectors_kb"), []byte("garbage\n"), 0o644))

	assert.Equal(t, 32*1024, kernelMaxTransfer(sysfs, "nvme0"))
}

func TestOpenNVMeMissing(t *testing.T) {
	t.Parallel()

	_, err := OpenNVMe("/nonexistent-nvme-device")
	require.Error(t, err)
	require.ErrorContains(t, err, "/nonexistent-nvme-device")
}

func TestOpenControllerReadOnlyCloexec(t *testing.T) {
	t.Parallel()

	dev, err := openController("/dev/null")
	require.NoError(t, err)

	defer dev.Close() //nolint:errcheck

	fdFlags, err := unix.FcntlInt(uintptr(dev.fd), unix.F_GETFD, 0)
	require.NoError(t, err)
	assert.NotZero(t, fdFlags&unix.FD_CLOEXEC, "device fd must be close-on-exec")

	flFlags, err := unix.FcntlInt(uintptr(dev.fd), unix.F_GETFL, 0)
	require.NoError(t, err)
	assert.Equal(t, unix.O_RDONLY, flFlags&unix.O_ACCMODE, "device must be opened read-only")
}

func TestOpenControllerNotCharDevice(t *testing.T) {
	t.Parallel()

	regular := filepath.Join(t.TempDir(), "nvme0")
	require.NoError(t, os.WriteFile(regular, nil, 0o644))

	_, err := openController(regular)
	require.ErrorContains(t, err, "is not a character device")
}
