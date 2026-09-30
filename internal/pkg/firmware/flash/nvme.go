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
	"unsafe"

	"golang.org/x/sys/unix"
)

func init() {
	Register(ProtocolNVMe, nvmeFlasher{})
}

type nvmeFlasher struct {
	// open is a test seam; nil uses openNVMe.
	open func(devPath string) (nvmeDevice, error)
}

// nvmeDevice is the subset of NVMeDevice used by the flasher.
type nvmeDevice interface {
	DownloadFirmware(ctx context.Context, image []byte) error
	Commit(ctx context.Context) (Activation, error)
	Close() error
}

// Flash implements Flasher.
func (n nvmeFlasher) Flash(ctx context.Context, target Target, payload []byte) (Activation, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("firmware flash canceled: %w", err)
	}

	open := n.open
	if open == nil {
		open = openNVMe
	}

	dev, err := open(target.DevPath)
	if err != nil {
		return 0, err
	}

	defer dev.Close() //nolint:errcheck

	if err := dev.DownloadFirmware(ctx, payload); err != nil {
		return 0, err
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
	opcodeIdentify         = 0x06
	opcodeFirmwareCommit   = 0x10
	opcodeFirmwareDownload = 0x11
)

// Identify Controller CNS and byte offsets of MDTS and FWUG in the 4 KiB response.
const (
	cnsIdentifyController     = 0x01
	identifyControllerDataLen = 4096
	identifyMDTSOffset        = 77
	identifyFWUGOffset        = 319
)

// NVMe status after the kernel's `>>1`: SC in bits 7:0, SCT in bits 10:8 — hence
// the 0x7ff mask. "Requires reset" statuses are Commit-only; see Commit.
const (
	statusSuccess            = 0x000
	statusFWNeedsConvReset   = 0x10b
	statusFWNeedsSubsysReset = 0x110
	statusFWNeedsReset       = 0x111
)

// Used when FWUG is 0 ("no info") or 0xff ("no restriction").
const defaultDownloadChunkSize = 4096

// Page size for MDTS; CAP.MPSMIN isn't exposed via admin ioctls, so assume the
// near-universal 4 KiB (what fwupd assumes).
const nvmePageSize = 4096

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

	// ioctl is a test seam; nil uses the real syscall. The real syscall reads
	// via cmd.addr and ignores buf; the test fake writes response data into
	// buf directly to avoid an unsafe-pointer round-trip under checkptr.
	ioctl func(fd int, cmd *adminCmd, buf []byte) (status uintptr, errno unix.Errno)

	fwug       uint8
	mdts       uint8
	identified bool
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

// realIoctl dispatches NVME_IOCTL_ADMIN_CMD; the kernel reads the data buffer
// via cmd.addr, so buf is unused here.
func realIoctl(fd int, cmd *adminCmd, _ []byte) (uintptr, unix.Errno) {
	status, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), nvmeIoctlAdminCmd, uintptr(unsafe.Pointer(cmd)))

	return status, errno
}

// adminPassthru issues an admin command and returns the NVMe status (SCT|SC,
// masked with 0x7ff). errno maps to (0, err); a non-zero status maps to
// (status, err) so callers can accept specific statuses without unwrapping.
func (d *NVMeDevice) adminPassthru(cmd *adminCmd, buf []byte) (uint16, error) {
	ioctl := d.ioctl
	if ioctl == nil {
		ioctl = realIoctl
	}

	rawStatus, errno := ioctl(d.fd, cmd, buf)
	if errno != 0 {
		return 0, fmt.Errorf("admin command 0x%02x failed: %w", cmd.opcode, errno)
	}

	sc := uint16(rawStatus & 0x7ff)
	if sc == statusSuccess {
		return 0, nil
	}

	return sc, fmt.Errorf("admin command 0x%02x failed with NVMe status 0x%03x", cmd.opcode, sc)
}

// DownloadFirmware transfers the image in chunks (Firmware Image Download, 0x11).
func (d *NVMeDevice) DownloadFirmware(ctx context.Context, image []byte) error {
	if len(image) == 0 {
		return errors.New("empty firmware image")
	}

	chunkSize, err := d.chunkSize()
	if err != nil {
		return fmt.Errorf("error determining download chunk size: %w", err)
	}

	cmd := adminCmd{
		opcode:    opcodeFirmwareDownload,
		timeoutMS: 120_000,
	}

	for offset := 0; offset < len(image); offset += chunkSize {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("firmware download canceled at offset %d: %w", offset, err)
		}

		chunk := firmwareChunk(image, offset, chunkSize)

		cmd.addr = uint64(uintptr(unsafe.Pointer(&chunk[0])))
		cmd.dataLen = uint32(len(chunk))
		cmd.cdw10 = uint32(len(chunk)>>2) - 1 // number of dwords, 0-based
		cmd.cdw11 = uint32(offset >> 2)       // offset in dwords

		if _, err := d.adminPassthru(&cmd, chunk); err != nil {
			return fmt.Errorf("error downloading firmware chunk at offset %d: %w", offset, err)
		}

		// cmd.addr is a uint64 the GC doesn't track; keep chunk live past the ioctl.
		runtime.KeepAlive(chunk)
	}

	return nil
}

// firmwareChunk returns image[offset:offset+chunkSize], padding a short tail
// with 0xff (matching fwupd). Full chunkSize padding — not just dword — is
// required because NUMD and OFST must be FWUG-aligned when FWUG > 1.
func firmwareChunk(image []byte, offset, chunkSize int) []byte {
	end := min(offset+chunkSize, len(image))
	actual := image[offset:end]

	if len(actual) == chunkSize {
		return actual
	}

	padded := make([]byte, chunkSize)
	copy(padded, actual)

	for i := len(actual); i < len(padded); i++ {
		padded[i] = 0xff
	}

	return padded
}

// Caps the MDTS exponent before the (1<<mdts) shift so a bogus Identify
// response can't overflow int. 1<<30 * 4 KiB = 4 TiB; real controllers report 5-10.
const mdtsExponentCap = 30

// chunkSize returns the download transfer size honoring FWUG and capped by MDTS.
// Returns an error when a FWUG-aligned chunk can't fit under the MDTS limit.
func (d *NVMeDevice) chunkSize() (int, error) {
	if !d.identified {
		if err := d.identify(); err != nil {
			return 0, err
		}
	}

	size := defaultDownloadChunkSize
	if d.fwug != 0 && d.fwug != 0xff {
		size = int(d.fwug) * defaultDownloadChunkSize
	}

	if d.mdts != 0 && d.mdts <= mdtsExponentCap {
		maxBytes := (1 << d.mdts) * nvmePageSize
		if size > maxBytes {
			return 0, fmt.Errorf("FWUG-aligned chunk size %d exceeds MDTS limit %d bytes", size, maxBytes)
		}
	}

	return size, nil
}

// identify runs Identify Controller (0x06, CNS=1) and caches MDTS and FWUG.
func (d *NVMeDevice) identify() error {
	data := make([]byte, identifyControllerDataLen)

	cmd := adminCmd{
		opcode:    opcodeIdentify,
		addr:      uint64(uintptr(unsafe.Pointer(&data[0]))),
		dataLen:   identifyControllerDataLen,
		cdw10:     cnsIdentifyController,
		timeoutMS: 60_000,
	}

	if _, err := d.adminPassthru(&cmd, data); err != nil {
		return fmt.Errorf("identify controller: %w", err)
	}

	runtime.KeepAlive(data)

	d.mdts = data[identifyMDTSOffset]
	d.fwug = data[identifyFWUGOffset]
	d.identified = true

	return nil
}

// Commit stages the image into a controller-chosen slot (Firmware Commit, 0x10,
// CA=0b001, FS=0) and reports the reset needed to activate it. Plain success
// means the next controller-level reset, which any reboot (kexec included)
// performs; the three "requires reset" statuses are success with a stronger
// requirement: a conventional (PCIe) reset is not done by kexec, and an NVM
// subsystem reset needs a power cycle.
func (d *NVMeDevice) Commit(ctx context.Context) (Activation, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("firmware commit canceled: %w", err)
	}

	const commitAction = 0b001

	cmd := adminCmd{
		opcode:    opcodeFirmwareCommit,
		cdw10:     commitAction << 3,
		timeoutMS: 120_000,
	}

	status, err := d.adminPassthru(&cmd, nil)

	switch {
	case err == nil, status == statusFWNeedsReset:
		return ActivationReboot, nil
	case status == statusFWNeedsConvReset:
		return ActivationFullReboot, nil
	case status == statusFWNeedsSubsysReset:
		return ActivationPowerCycle, nil
	default:
		return 0, err
	}
}
