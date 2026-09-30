// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage // tests validate unexported struct layout against the kernel ABI.
package flash

import (
	"context"
	"fmt"
	"testing"
	"time"
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
// fwug and afiSequence let tests script the responses of Identify Controller
// and Get Log Page (Firmware Slot Information) commands.
type ioctlRecorder struct {
	commands []adminCmd
	status   uintptr
	errno    unix.Errno
	failOn   int

	fwug        uint8
	afiSequence []uint8
	afiIndex    int
}

// cmdBuffer materializes the caller's data buffer from cmd.addr. The caller
// (production DownloadFirmware / Identify / etc.) keeps the backing slice
// live for the duration of this ioctl call via runtime.KeepAlive.
func cmdBuffer(cmd *adminCmd) []byte {
	//nolint:govet // cmd.addr is a stable pointer kept live by the caller for the ioctl duration.
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(cmd.addr))), cmd.dataLen)
}

func (r *ioctlRecorder) ioctl(_ int, cmd *adminCmd) (uintptr, unix.Errno) {
	r.commands = append(r.commands, *cmd)

	if r.failOn == len(r.commands) {
		return 0, unix.EIO
	}

	switch cmd.opcode {
	case opcodeIdentify:
		cmdBuffer(cmd)[identifyFWUGOffset] = r.fwug
	case opcodeGetLogPage:
		// AFI defaults to 0 (FWUP clear) if no sequence is set; otherwise
		// step through the sequence, holding the final value for any
		// additional reads.
		var afi uint8

		if len(r.afiSequence) > 0 {
			if r.afiIndex >= len(r.afiSequence) {
				afi = r.afiSequence[len(r.afiSequence)-1]
			} else {
				afi = r.afiSequence[r.afiIndex]
				r.afiIndex++
			}
		}

		cmdBuffer(cmd)[0] = afi
	}

	return r.status, r.errno
}

// preFWUG returns a device that skips the Identify call with FWUG=1 (default
// 4 KiB chunks). Used by tests that don't exercise the Identify path.
func preFWUG(rec *ioctlRecorder) *NVMeDevice {
	return &NVMeDevice{ioctl: rec.ioctl, fwug: 1, fwugRead: true, sleep: func(time.Duration) {}}
}

func TestDownloadFirmwareChunking(t *testing.T) {
	t.Parallel()

	image := make([]byte, 2*defaultDownloadChunkSize+101)
	for i := range image {
		image[i] = byte(i%251 + 1)
	}

	rec := &ioctlRecorder{}
	dev := preFWUG(rec)

	require.NoError(t, dev.DownloadFirmware(context.Background(), image))
	// 3 download chunks + 1 FWUP get-log-page = 4 commands.
	require.Len(t, rec.commands, 4)

	for i, cmd := range rec.commands[:3] {
		assert.Equal(t, uint8(opcodeFirmwareDownload), cmd.opcode, "chunk %d opcode", i)
		assert.NotZero(t, cmd.addr, "chunk %d data address", i)
		assert.NotZero(t, cmd.timeoutMS, "chunk %d timeout", i)
	}

	// first chunk: full defaultDownloadChunkSize
	assert.Equal(t, uint32(defaultDownloadChunkSize), rec.commands[0].dataLen)
	assert.Equal(t, uint32(defaultDownloadChunkSize>>2)-1, rec.commands[0].cdw10)
	assert.Equal(t, uint32(0), rec.commands[0].cdw11)

	// second chunk: full, offset advances in dwords
	assert.Equal(t, uint32(defaultDownloadChunkSize), rec.commands[1].dataLen)
	assert.Equal(t, uint32(defaultDownloadChunkSize>>2)-1, rec.commands[1].cdw10)
	assert.Equal(t, uint32(defaultDownloadChunkSize>>2), rec.commands[1].cdw11)

	// final chunk: 101 bytes padded to 104 (a dword multiple)
	assert.Equal(t, uint32(104), rec.commands[2].dataLen)
	assert.Equal(t, uint32(104>>2)-1, rec.commands[2].cdw10)
	assert.Equal(t, uint32(2*defaultDownloadChunkSize>>2), rec.commands[2].cdw11)

	// trailing FWUP poll: Get Log Page for firmware slot info.
	assert.Equal(t, uint8(opcodeGetLogPage), rec.commands[3].opcode)
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

	// final chunk is padded to a dword boundary with 0xff
	tail := firmwareChunk(image, 2*defaultDownloadChunkSize, defaultDownloadChunkSize)
	require.Len(t, tail, 104)
	assert.Equal(t, image[2*defaultDownloadChunkSize:], tail[:101])
	assert.Equal(t, []byte{0xff, 0xff, 0xff}, tail[101:])
}

func TestFirmwareChunkSmall(t *testing.T) {
	t.Parallel()

	// a sub-dword image is padded to a single dword
	assert.Equal(t, []byte{1, 2, 3, 0xff}, firmwareChunk([]byte{1, 2, 3}, 0, defaultDownloadChunkSize))
}

func TestDownloadFirmwareSmallImage(t *testing.T) {
	t.Parallel()

	// sub-dword image; unpadded cdw10 would underflow to 0xffffffff.
	rec := &ioctlRecorder{}
	dev := preFWUG(rec)

	require.NoError(t, dev.DownloadFirmware(context.Background(), []byte{1, 2, 3}))
	// 1 download + 1 FWUP get-log-page.
	require.Len(t, rec.commands, 2)

	assert.Equal(t, uint32(4), rec.commands[0].dataLen)
	assert.Equal(t, uint32(0), rec.commands[0].cdw10) // (4>>2)-1, not 0xffffffff
	assert.Equal(t, uint32(0), rec.commands[0].cdw11)
}

func TestDownloadFirmwareEmpty(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{}
	dev := preFWUG(rec)

	require.Error(t, dev.DownloadFirmware(context.Background(), nil))
	assert.Empty(t, rec.commands)
}

func TestDownloadFirmwareError(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{failOn: 2}
	dev := preFWUG(rec)

	err := dev.DownloadFirmware(context.Background(), make([]byte, 2*defaultDownloadChunkSize))
	require.ErrorIs(t, err, unix.EIO)
	require.ErrorContains(t, err, fmt.Sprintf("offset %d", defaultDownloadChunkSize))
	require.Len(t, rec.commands, 2) // failed on the second chunk, no FWUP poll reached
}

func TestDownloadFirmwareCanceledBeforeStart(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{}
	dev := preFWUG(rec)

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
		fwug:     1,
		fwugRead: true,
		sleep:    func(time.Duration) {},
		ioctl: func(fd int, cmd *adminCmd) (uintptr, unix.Errno) {
			cancel() // cancel while the first chunk is in flight

			return rec.ioctl(fd, cmd)
		},
	}

	err := dev.DownloadFirmware(ctx, make([]byte, 2*defaultDownloadChunkSize))
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorContains(t, err, fmt.Sprintf("offset %d", defaultDownloadChunkSize))
	require.Len(t, rec.commands, 1) // the second chunk is never submitted; FWUP not reached
}

func TestCommit(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{}
	dev := &NVMeDevice{ioctl: rec.ioctl}

	require.NoError(t, dev.Commit(context.Background()))
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

	require.ErrorIs(t, dev.Commit(ctx), context.Canceled)
	assert.Empty(t, rec.commands)
}

func TestAdminPassthruStatuses(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		status  uintptr
		errno   unix.Errno
		wantErr string
	}{
		{name: "success", status: statusSuccess},
		{name: "fw needs conventional reset", status: statusFWNeedsConvReset},
		{name: "fw needs subsystem reset", status: statusFWNeedsSubsysReset},
		{name: "fw needs reset", status: statusFWNeedsReset},
		{name: "fw image error", status: 0x107, wantErr: "0x107"},
		{name: "errno failure", errno: unix.EIO, wantErr: "input/output"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := &ioctlRecorder{status: tc.status, errno: tc.errno}
			dev := &NVMeDevice{ioctl: rec.ioctl}

			err := dev.Commit(context.Background())

			if tc.wantErr == "" {
				require.NoError(t, err)

				return
			}

			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestChunkSize(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		fwug    uint8
		wantLen int
	}{
		{name: "no info (FWUG=0)", fwug: 0, wantLen: defaultDownloadChunkSize},
		{name: "FWUG=1", fwug: 1, wantLen: defaultDownloadChunkSize},
		{name: "FWUG=2", fwug: 2, wantLen: 2 * defaultDownloadChunkSize},
		{name: "no restriction (FWUG=0xff)", fwug: 0xff, wantLen: defaultDownloadChunkSize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := &ioctlRecorder{fwug: tc.fwug}
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

func TestDownloadFirmwareLargerChunkFromFWUG(t *testing.T) {
	t.Parallel()

	// FWUG=2 means the controller requires 8 KiB chunks; a 12 KiB image should
	// produce 2 download commands (8 KiB then 4 KiB padded), plus Identify and
	// the trailing FWUP poll.
	rec := &ioctlRecorder{fwug: 2}
	dev := &NVMeDevice{ioctl: rec.ioctl, sleep: func(time.Duration) {}}

	require.NoError(t, dev.DownloadFirmware(context.Background(), make([]byte, 3*defaultDownloadChunkSize)))
	require.Len(t, rec.commands, 4)

	assert.Equal(t, uint8(opcodeIdentify), rec.commands[0].opcode)
	assert.Equal(t, uint8(opcodeFirmwareDownload), rec.commands[1].opcode)
	assert.Equal(t, uint32(2*defaultDownloadChunkSize), rec.commands[1].dataLen)
	assert.Equal(t, uint8(opcodeFirmwareDownload), rec.commands[2].opcode)
	assert.Equal(t, uint32(defaultDownloadChunkSize), rec.commands[3-1].dataLen)
	assert.Equal(t, uint8(opcodeGetLogPage), rec.commands[3].opcode)
}

func TestWaitForFWUPImmediate(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{} // afi=0 by default: FWUP clear from the start
	dev := &NVMeDevice{ioctl: rec.ioctl, sleep: func(time.Duration) {}}

	require.NoError(t, dev.waitForFWUP(context.Background()))
	require.Len(t, rec.commands, 1)

	cmd := rec.commands[0]
	assert.Equal(t, uint8(opcodeGetLogPage), cmd.opcode)
	assert.Equal(t, uint32(firmwareSlotLogSize), cmd.dataLen)
	// LID in low byte of cdw10; NUMDL (dword count - 1) in high 16 bits.
	assert.Equal(t, uint32(logIDFirmwareSlot), cmd.cdw10&0xff)
	assert.Equal(t, uint32(firmwareSlotLogSize>>2-1), (cmd.cdw10>>16)&0xffff)
}

func TestWaitForFWUPPolls(t *testing.T) {
	t.Parallel()

	// AFI reports FWUP set for the first two polls, then clears.
	rec := &ioctlRecorder{afiSequence: []uint8{firmwareSlotAFIFWUP, firmwareSlotAFIFWUP, 0}}
	dev := &NVMeDevice{ioctl: rec.ioctl, sleep: func(time.Duration) {}}

	require.NoError(t, dev.waitForFWUP(context.Background()))
	require.Len(t, rec.commands, 3)
}

func TestWaitForFWUPTimeout(t *testing.T) {
	t.Parallel()

	// AFI never clears; the short fwupTimeout drives a real deadline expiry.
	rec := &ioctlRecorder{afiSequence: make([]uint8, 1024)}
	for i := range rec.afiSequence {
		rec.afiSequence[i] = firmwareSlotAFIFWUP
	}

	dev := &NVMeDevice{
		ioctl:       rec.ioctl,
		sleep:       func(time.Duration) {},
		fwupTimeout: time.Millisecond,
	}

	err := dev.waitForFWUP(context.Background())
	require.Error(t, err)
	require.ErrorContains(t, err, "timed out waiting for firmware update")
}

func TestWaitForFWUPCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	rec := &ioctlRecorder{afiSequence: []uint8{firmwareSlotAFIFWUP, firmwareSlotAFIFWUP}}
	dev := &NVMeDevice{
		ioctl: func(fd int, cmd *adminCmd) (uintptr, unix.Errno) {
			r, e := rec.ioctl(fd, cmd)

			cancel() // cancel after the first Get Log Page returns

			return r, e
		},
		sleep: func(time.Duration) {},
	}

	err := dev.waitForFWUP(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestWaitForFWUPLogError(t *testing.T) {
	t.Parallel()

	rec := &ioctlRecorder{failOn: 1}
	dev := &NVMeDevice{ioctl: rec.ioctl, sleep: func(time.Duration) {}}

	err := dev.waitForFWUP(context.Background())
	require.ErrorIs(t, err, unix.EIO)
	require.ErrorContains(t, err, "getting firmware slot info")
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
