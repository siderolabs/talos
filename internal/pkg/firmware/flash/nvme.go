// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package flash implements native device firmware flashing.
package flash

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

func init() {
	Register(ProtocolNVMe, nvmeFlasher{open: openNVMe})
}

// nvmeFlasher implements Flasher for the org.nvmexpress protocol.
type nvmeFlasher struct {
	// open is a seam so tests can substitute a fake device.
	open func(devPath string) (nvmeDevice, error)
}

// nvmeDevice is the subset of NVMeDevice used by the flasher.
type nvmeDevice interface {
	DownloadFirmware(ctx context.Context, image []byte) error
	Commit(ctx context.Context) error
	Close() error
}

// Flash implements Flasher.
func (n nvmeFlasher) Flash(ctx context.Context, target Target, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("firmware flash canceled: %w", err)
	}

	dev, err := n.open(target.DevPath)
	if err != nil {
		return err
	}

	defer dev.Close() //nolint:errcheck

	if err := dev.DownloadFirmware(ctx, payload); err != nil {
		return err
	}

	return dev.Commit(ctx)
}

func openNVMe(devPath string) (nvmeDevice, error) {
	return OpenNVMe(devPath)
}

// NVME_IOCTL_ADMIN_CMD = _IOWR('N', 0x41, struct nvme_admin_cmd).
const nvmeIoctlAdminCmd = 0xc0484e41

// NVMe admin opcodes.
const (
	opcodeGetLogPage       = 0x02
	opcodeIdentify         = 0x06
	opcodeFirmwareCommit   = 0x10
	opcodeFirmwareDownload = 0x11
)

// Identify Controller CNS value and the byte offset of FWUG in the returned
// 4 KiB data structure (NVMe base spec).
const (
	cnsIdentifyController     = 0x01
	identifyControllerDataLen = 4096
	identifyFWUGOffset        = 319
)

// Get Log Page: firmware slot information (LID 03h, 512 bytes). AFI byte 0
// carries the FWUP bit at 0x10 (fwupd convention observed on real drives).
const (
	logIDFirmwareSlot              = 0x03
	firmwareSlotLogSize            = 512
	firmwareSlotAFIFWUP            = 0x10
	firmwareUpdateWaitPollInterval = 100 * time.Millisecond
	firmwareUpdateWaitTimeout      = 10 * time.Second
)

// NVMe status codes (SCT|SC, masked with 0x3ff) which are successful for firmware commands.
//
// The "requires reset" statuses mean the firmware was staged and activates at next reset.
const (
	statusSuccess            = 0x000
	statusFWNeedsConvReset   = 0x10b
	statusFWNeedsSubsysReset = 0x110
	statusFWNeedsReset       = 0x111
)

// defaultDownloadChunkSize is used when the controller advertises no FWUG
// restriction (FWUG=0 "no info" or FWUG=0xFF "no restriction").
const defaultDownloadChunkSize = 4096

// adminCmd mirrors struct nvme_admin_cmd from <linux/nvme_ioctl.h> (72 bytes).
type adminCmd struct {
	opcode      uint8
	flags       uint8
	rsvd1       uint16
	nsid        uint32
	cdw2        uint32
	cdw3        uint32
	metadata    uint64
	addr        uint64
	metadataLen uint32
	dataLen     uint32
	cdw10       uint32
	cdw11       uint32
	cdw12       uint32
	cdw13       uint32
	cdw14       uint32
	cdw15       uint32
	timeoutMS   uint32
	result      uint32
}

// NVMeDevice flashes firmware on an NVMe device via admin passthrough ioctls.
type NVMeDevice struct {
	fd int

	// ioctl is a seam for tests; nil uses the real syscall. The buf argument
	// is the Go slice whose address is in cmd.addr; the real syscall ignores
	// it, but the test fake writes response data into it directly to avoid
	// unsafe pointer round-tripping that trips checkptr under -race.
	ioctl func(fd int, cmd *adminCmd, buf []byte) (status uintptr, errno unix.Errno)

	// sleep is a seam for tests; nil uses time.Sleep.
	sleep func(d time.Duration)

	// fwupTimeout overrides firmwareUpdateWaitTimeout when nonzero; used by tests.
	fwupTimeout time.Duration

	fwug     uint8
	fwugRead bool
}

// OpenNVMe opens an NVMe device (namespace block device or controller character device).
func OpenNVMe(devPath string) (*NVMeDevice, error) {
	fd, err := unix.Open(devPath, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("error opening %q: %w", devPath, err)
	}

	return &NVMeDevice{fd: fd}, nil
}

// Close closes the device.
func (d *NVMeDevice) Close() error {
	return unix.Close(d.fd)
}

func (d *NVMeDevice) adminPassthru(cmd *adminCmd, buf []byte) error {
	ioctl := d.ioctl
	if ioctl == nil {
		ioctl = func(fd int, cmd *adminCmd, _ []byte) (uintptr, unix.Errno) {
			status, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), nvmeIoctlAdminCmd, uintptr(unsafe.Pointer(cmd)))

			return status, errno
		}
	}

	status, errno := ioctl(d.fd, cmd, buf)
	if errno != 0 {
		return fmt.Errorf("admin command 0x%02x failed: %w", cmd.opcode, errno)
	}

	// kernels prior to 4.10 return the raw NVMe status as the ioctl return
	// value rather than errno; the mask covers both encodings.
	switch status & 0x3ff {
	case statusSuccess, statusFWNeedsConvReset, statusFWNeedsSubsysReset, statusFWNeedsReset:
		return nil
	default:
		return fmt.Errorf("admin command 0x%02x failed with NVMe status 0x%03x", cmd.opcode, status&0x3ff)
	}
}

// DownloadFirmware transfers the firmware image to the controller in chunks
// (Firmware Image Download, opcode 0x11) and waits for the controller to
// finish processing the image before returning.
func (d *NVMeDevice) DownloadFirmware(ctx context.Context, image []byte) error {
	if len(image) == 0 {
		return errors.New("empty firmware image")
	}

	chunkSize, err := d.chunkSize()
	if err != nil {
		return fmt.Errorf("error determining download chunk size: %w", err)
	}

	for offset := 0; offset < len(image); offset += chunkSize {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("firmware download canceled at offset %d: %w", offset, err)
		}

		chunk := firmwareChunk(image, offset, chunkSize)

		cmd := adminCmd{
			opcode:    opcodeFirmwareDownload,
			addr:      uint64(uintptr(unsafe.Pointer(&chunk[0]))),
			dataLen:   uint32(len(chunk)),
			cdw10:     uint32(len(chunk)>>2) - 1, // number of dwords, 0-based
			cdw11:     uint32(offset >> 2),       // offset in dwords
			timeoutMS: 120_000,
		}

		if err := d.adminPassthru(&cmd, chunk); err != nil {
			return fmt.Errorf("error downloading firmware chunk at offset %d: %w", offset, err)
		}

		// cmd.addr is a uint64 the GC doesn't track; keep chunk live past the ioctl.
		runtime.KeepAlive(chunk)
	}

	return d.waitForFWUP(ctx)
}

// firmwareChunk returns a dword-aligned chunk of image starting at offset (at
// most chunkSize bytes), padding a sub-dword tail with 0xff (matching fwupd).
// NVMe requires dword-aligned transfers; an unpadded sub-dword tail would
// underflow cdw10.
func firmwareChunk(image []byte, offset, chunkSize int) []byte {
	chunk := image[offset:min(offset+chunkSize, len(image))]

	if rem := len(chunk) % 4; rem != 0 {
		padded := make([]byte, len(chunk)+4-rem)
		copy(padded, chunk)

		for i := len(chunk); i < len(padded); i++ {
			padded[i] = 0xff
		}

		return padded
	}

	return chunk
}

// chunkSize returns the firmware download transfer size honoring the
// controller-advertised FWUG (Firmware Update Granularity). FWUG=0 ("no
// information") and FWUG=0xff ("no restriction") both fall back to the 4 KiB
// default; other values scale by 4 KiB.
func (d *NVMeDevice) chunkSize() (int, error) {
	if !d.fwugRead {
		fwug, err := d.identifyFWUG()
		if err != nil {
			return 0, err
		}

		d.fwug = fwug
		d.fwugRead = true
	}

	if d.fwug == 0 || d.fwug == 0xff {
		return defaultDownloadChunkSize, nil
	}

	return int(d.fwug) * defaultDownloadChunkSize, nil
}

// identifyFWUG issues Identify Controller (opcode 0x06, CNS=1) and returns the
// FWUG byte at offset 319 of the returned 4 KiB structure.
func (d *NVMeDevice) identifyFWUG() (uint8, error) {
	data := make([]byte, identifyControllerDataLen)

	cmd := adminCmd{
		opcode:    opcodeIdentify,
		addr:      uint64(uintptr(unsafe.Pointer(&data[0]))),
		dataLen:   identifyControllerDataLen,
		cdw10:     cnsIdentifyController,
		timeoutMS: 60_000,
	}

	if err := d.adminPassthru(&cmd, data); err != nil {
		return 0, fmt.Errorf("identify controller: %w", err)
	}

	runtime.KeepAlive(data)

	return data[identifyFWUGOffset], nil
}

// waitForFWUP polls the firmware slot information log until the FWUP bit
// clears, indicating the controller has finished processing the downloaded
// image. Some controllers reject Firmware Commit while FWUP is set; fwupd
// added this poll for the same reason.
func (d *NVMeDevice) waitForFWUP(ctx context.Context) error {
	timeout := d.fwupTimeout
	if timeout == 0 {
		timeout = firmwareUpdateWaitTimeout
	}

	deadline := time.Now().Add(timeout)

	sleep := d.sleep
	if sleep == nil {
		sleep = time.Sleep
	}

	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("waiting for firmware update to complete: %w", err)
		}

		data := make([]byte, firmwareSlotLogSize)

		cmd := adminCmd{
			opcode:    opcodeGetLogPage,
			addr:      uint64(uintptr(unsafe.Pointer(&data[0]))),
			dataLen:   firmwareSlotLogSize,
			cdw10:     uint32(logIDFirmwareSlot) | (uint32(firmwareSlotLogSize>>2-1) << 16),
			timeoutMS: 30_000,
		}

		if err := d.adminPassthru(&cmd, data); err != nil {
			return fmt.Errorf("getting firmware slot info: %w", err)
		}

		runtime.KeepAlive(data)

		if data[0]&firmwareSlotAFIFWUP == 0 {
			return nil
		}

		if time.Now().After(deadline) {
			return errors.New("timed out waiting for firmware update to complete")
		}

		sleep(firmwareUpdateWaitPollInterval)
	}
}

// Commit stages the downloaded image into a controller-chosen slot and requests
// activation at the next reset (Firmware Commit, opcode 0x10, CA=0b001, FS=0).
func (d *NVMeDevice) Commit(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("firmware commit canceled: %w", err)
	}

	const commitAction = 0b001 // replace image in slot and activate at next reset

	cmd := adminCmd{
		opcode:    opcodeFirmwareCommit,
		cdw10:     commitAction << 3,
		timeoutMS: 120_000,
	}

	if err := d.adminPassthru(&cmd, nil); err != nil {
		return fmt.Errorf("error committing firmware: %w", err)
	}

	return nil
}
