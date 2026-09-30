// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux && (amd64 || arm64 || loong64 || riscv64 || s390x)

// The ioctl request numbers below use the asm-generic _IOC encoding, which the
// architectures above share; powerpc and mips encode the direction bits
// differently, so the NVMe flasher is not built there.

package flash

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
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
	Subsystem() string
	DownloadFirmware(ctx context.Context, image []byte) error
	Commit(ctx context.Context) (Activation, error)
	Close() error
}

// Flash implements Flasher.
//
// Flashes of one NVM subsystem are serialized: two targets can resolve to the
// same controller (two namespaces, a namespace and its partition) or to two
// controllers of one subsystem (multipath), and a second download would hit
// the first one's staged image and reset the controller under it. Each flash
// still downloads and commits the image, so callers should flash a subsystem
// once.
func (n nvmeFlasher) Flash(ctx context.Context, target Target, payload []byte) (Activation, error) {
	if err := ctx.Err(); err != nil {
		return ActivationNone, fmt.Errorf("firmware flash canceled: %w", err)
	}

	open := n.open
	if open == nil {
		open = openNVMe
	}

	dev, err := open(target.DevPath)
	if err != nil {
		return ActivationNone, err
	}

	defer dev.Close() //nolint:errcheck

	unlock, err := lockSubsystem(ctx, dev.Subsystem())
	if err != nil {
		return ActivationNone, fmt.Errorf("firmware flash canceled waiting for another flash of %s: %w", dev.Subsystem(), err)
	}

	defer unlock()

	if err := dev.DownloadFirmware(ctx, payload); err != nil {
		return ActivationNone, err
	}

	return dev.Commit(ctx)
}

func openNVMe(devPath string) (nvmeDevice, error) {
	return OpenNVMe(devPath)
}

// subsystemLocks holds a one-slot semaphore per NVM subsystem name; see Flash.
var subsystemLocks = struct {
	sync.Mutex

	m map[string]chan struct{}
}{m: map[string]chan struct{}{}}

// lockSubsystem waits until no other flash of the subsystem is running, or ctx
// is done, and returns the function that releases it.
func lockSubsystem(ctx context.Context, name string) (func(), error) {
	subsystemLocks.Lock()

	sem, ok := subsystemLocks.m[name]
	if !ok {
		sem = make(chan struct{}, 1)
		subsystemLocks.m[name] = sem
	}

	subsystemLocks.Unlock()

	select {
	case sem <- struct{}{}:
		return func() { <-sem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Request numbers from <linux/nvme_ioctl.h>:
// NVME_IOCTL_ADMIN_CMD = _IOWR('N', 0x41, struct nvme_admin_cmd) and
// NVME_IOCTL_RESET = _IO('N', 0x44), in the asm-generic encoding (see the
// build constraint). The reset is only accepted on the controller character
// device.
const (
	nvmeIoctlAdminCmd = 0xc0484e41
	nvmeIoctlReset    = 0x4e44
)

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
	statusOverlappingRange   = 0x114
)

// statusNames labels the statuses a firmware update is likely to hit, so an
// error reads as more than a hex code. Unlisted statuses are reported as hex only.
var statusNames = map[uint16]string{
	0x001:                  "invalid command opcode",
	0x002:                  "invalid field in command",
	0x004:                  "data transfer error",
	0x006:                  "internal error",
	0x106:                  "invalid firmware slot",
	0x107:                  "invalid firmware image",
	0x112:                  "firmware activation requires maximum time violation",
	0x113:                  "firmware activation prohibited",
	statusOverlappingRange: "overlapping range",
}

// Used when FWUG is 0 ("no info") or 0xff ("no restriction"), as fwupd and
// nvme-cli do: larger transfers would be allowed for 0xff, but small ones are
// what drives are known to accept.
const defaultDownloadChunkSize = 4096

// Page size for MDTS; CAP.MPSMIN isn't exposed via admin ioctls, so assume the
// near-universal 4 KiB (what fwupd assumes).
const nvmePageSize = 4096

// adminCmd mirrors struct nvme_admin_cmd from <linux/nvme_ioctl.h> (72 bytes).
//
// addr is the kernel's __u64 declared as a pointer, the way unix.Iovec.Base is:
// the GC then keeps the data buffer alive and the field follows the buffer if
// it is ever moved, instead of relying on escape analysis to keep the buffer
// off the stack. The kernel reads the same 8 bytes either way, which is why the
// file is built for 64-bit architectures only (also asserted below).
type adminCmd struct {
	opcode      uint8
	flags       uint8
	rsvd1       uint16
	nsid        uint32
	cdw2        uint32
	cdw3        uint32
	metadata    uint64
	addr        unsafe.Pointer
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

// adminCmd.addr must be 8 bytes wide; this fails to compile on 32-bit platforms.
const _ = unsafe.Sizeof(unsafe.Pointer(nil)) - 8

// NVMeDevice flashes firmware on an NVMe controller via admin passthrough ioctls.
type NVMeDevice struct {
	fd int

	// ioctl is a test seam; nil uses the real syscall. For nvmeIoctlAdminCmd
	// the real syscall reads the data buffer via cmd.addr and ignores buf; the
	// test fake writes response data into buf directly. For nvmeIoctlReset
	// cmd and buf are nil.
	ioctl func(fd int, req uint, cmd *adminCmd, buf []byte) (status uintptr, errno unix.Errno)

	// maxTransfer is the kernel's transfer limit for the controller's queues
	// in bytes (max_hw_sectors_kb); 0 means unknown.
	maxTransfer int

	// subsystem names the NVM subsystem the controller belongs to (or the
	// controller itself, if that is unknown); see Subsystem.
	subsystem string

	fwug       uint8
	mdts       uint8
	identified bool
}

// Roots used to resolve a device path to its controller; see controllerFor.
const (
	sysfsRoot = "/sys"
	devRoot   = "/dev"
)

// OpenNVMe opens the NVMe controller behind devPath, which may be a controller
// character device (/dev/nvme0), a namespace block device (/dev/nvme0n1) or a
// partition of one.
//
// Admin commands always go to the controller character device: with native
// multipath, a namespace block device picks a path on every ioctl, so the
// firmware chunks and the commit could be spread across controllers.
func OpenNVMe(devPath string) (*NVMeDevice, error) {
	ctrl, err := controllerFor(sysfsRoot, devPath)
	if err != nil {
		return nil, err
	}

	dev, err := openController(filepath.Join(devRoot, ctrl))
	if err != nil {
		return nil, err
	}

	dev.maxTransfer = kernelMaxTransfer(sysfsRoot, ctrl)
	dev.subsystem = subsystemFor(sysfsRoot, ctrl)

	return dev, nil
}

// Subsystem returns the name of the controller's NVM subsystem (e.g.
// "nvme-subsys0"), or the controller name if it could not be determined.
func (d *NVMeDevice) Subsystem() string {
	return d.subsystem
}

// subsystemFor finds the NVM subsystem that links to the controller; sysfs has
// no link in the other direction.
func subsystemFor(sysfs, ctrl string) string {
	matches, err := filepath.Glob(filepath.Join(sysfs, "class", "nvme-subsystem", "nvme-subsys*", ctrl))
	if err != nil || len(matches) == 0 {
		return ctrl
	}

	return filepath.Base(filepath.Dir(matches[0]))
}

// openController opens a controller character device read-only: admin
// passthrough needs CAP_SYS_ADMIN, not write access.
func openController(path string) (*NVMeDevice, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("error opening %q: %w", path, err)
	}

	var st unix.Stat_t

	if err := unix.Fstat(fd, &st); err != nil {
		unix.Close(fd) //nolint:errcheck

		return nil, fmt.Errorf("error stating %q: %w", path, err)
	}

	if st.Mode&unix.S_IFMT != unix.S_IFCHR {
		unix.Close(fd) //nolint:errcheck

		return nil, fmt.Errorf("%q is not a character device", path)
	}

	return &NVMeDevice{fd: fd}, nil
}

var (
	nvmeControllerName = regexp.MustCompile(`^nvme\d+$`)
	nvmeSubsystemName  = regexp.MustCompile(`^nvme-subsys\d+$`)
	nvmeNamespaceName  = regexp.MustCompile(`^nvme\d+(c\d+)?n\d+$`)
)

// controllerFor resolves devPath to the name of its NVMe controller (e.g.
// "nvme0") through sysfs.
func controllerFor(sysfs, devPath string) (string, error) {
	var st unix.Stat_t

	if err := unix.Stat(devPath, &st); err != nil {
		return "", fmt.Errorf("error stating %q: %w", devPath, err)
	}

	node := fmt.Sprintf("%d:%d", unix.Major(st.Rdev), unix.Minor(st.Rdev))

	switch st.Mode & unix.S_IFMT {
	case unix.S_IFCHR:
		return controllerForNode(sysfs, true, node)
	case unix.S_IFBLK:
		return controllerForNode(sysfs, false, node)
	default:
		return "", fmt.Errorf("%q is not a block or character device", devPath)
	}
}

// controllerForNode resolves a device number through /sys/dev. A character
// device must be the controller itself. A block device's "device" link points
// at its controller, or at the NVM subsystem when native multipath is enabled;
// a firmware image belongs to the subsystem, so any of its controllers can
// flash it as long as a single one does all of it, and a live one is used.
func controllerForNode(sysfs string, char bool, node string) (string, error) {
	kind := "block"

	if char {
		kind = "char"
	}

	dir, err := filepath.EvalSymlinks(filepath.Join(sysfs, "dev", kind, node))
	if err != nil {
		return "", fmt.Errorf("error resolving %s device %s in sysfs: %w", kind, node, err)
	}

	if char {
		if name := filepath.Base(dir); nvmeControllerName.MatchString(name) {
			return name, nil
		}

		return "", fmt.Errorf("character device %s is not an NVMe controller", node)
	}

	// a partition has no "device" link; its parent (the whole namespace) does.
	if _, err := os.Stat(filepath.Join(dir, "partition")); err == nil {
		dir = filepath.Dir(dir)
	}

	target, err := filepath.EvalSymlinks(filepath.Join(dir, "device"))
	if err != nil {
		return "", fmt.Errorf("error resolving the device of block device %s: %w", node, err)
	}

	name := filepath.Base(target)

	switch {
	case nvmeControllerName.MatchString(name):
		return name, nil
	case nvmeSubsystemName.MatchString(name):
		return liveController(target)
	default:
		return "", fmt.Errorf("block device %s is not an NVMe namespace", node)
	}
}

// liveController picks a controller of a multipath NVM subsystem directory
// whose state is "live". A controller that is connecting, resetting or dead
// could fail admin commands part-way, so there is no fallback to one.
func liveController(subsysDir string) (string, error) {
	entries, err := os.ReadDir(subsysDir)
	if err != nil {
		return "", fmt.Errorf("error listing %q: %w", subsysDir, err)
	}

	names := make([]string, 0, len(entries))

	for _, e := range entries {
		if nvmeControllerName.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}

	if len(names) == 0 {
		return "", fmt.Errorf("no controllers found in %q", subsysDir)
	}

	for _, name := range names {
		state, err := os.ReadFile(filepath.Join(subsysDir, name, "state"))
		if err == nil && strings.TrimSpace(string(state)) == "live" {
			return name, nil
		}
	}

	return "", fmt.Errorf("no live controller in %q (found %s)", subsysDir, strings.Join(names, ", "))
}

// kernelMaxTransfer returns the smallest max_hw_sectors_kb of the controller's
// namespaces in bytes, or 0 when none is readable. The kernel applies the same
// limit to the admin queue, and rejects passthrough buffers above it with
// EINVAL; it is MDTS lowered by the driver's own cap and the platform's DMA
// limits, which Identify Controller cannot reveal. The admin queue's limit is
// not in sysfs, so a controller without namespaces reports 0 (see download).
func kernelMaxTransfer(sysfs, ctrl string) int {
	ctrlDir := filepath.Join(sysfs, "class", "nvme", ctrl)

	entries, err := os.ReadDir(ctrlDir)
	if err != nil {
		return 0
	}

	limit := 0

	for _, e := range entries {
		if !nvmeNamespaceName.MatchString(e.Name()) {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(ctrlDir, e.Name(), "queue", "max_hw_sectors_kb"))
		if err != nil {
			continue
		}

		kb, err := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err != nil || kb <= 0 {
			continue
		}

		if limit == 0 || kb*1024 < limit {
			limit = kb * 1024
		}
	}

	return limit
}

// Close closes the device.
func (d *NVMeDevice) Close() error {
	return unix.Close(d.fd)
}

// realIoctl issues an NVMe ioctl. For nvmeIoctlAdminCmd the kernel reads the
// data buffer via cmd.addr, so buf is unused; for nvmeIoctlReset cmd is nil.
func realIoctl(fd int, req uint, cmd *adminCmd, _ []byte) (uintptr, unix.Errno) {
	status, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(cmd)))

	return status, errno
}

func (d *NVMeDevice) doIoctl(req uint, cmd *adminCmd, buf []byte) (uintptr, unix.Errno) {
	if d.ioctl != nil {
		return d.ioctl(d.fd, req, cmd, buf)
	}

	return realIoctl(d.fd, req, cmd, buf)
}

// adminPassthru issues an admin command and returns the NVMe status (SCT|SC,
// masked with 0x7ff). errno maps to (0, err); a non-zero status maps to
// (status, err) so callers can accept specific statuses without unwrapping.
func (d *NVMeDevice) adminPassthru(cmd *adminCmd, buf []byte) (uint16, error) {
	rawStatus, errno := d.doIoctl(nvmeIoctlAdminCmd, cmd, buf)
	if errno != 0 {
		return 0, fmt.Errorf("admin command 0x%02x failed: %w", cmd.opcode, errno)
	}

	sc := uint16(rawStatus & 0x7ff)
	if sc == statusSuccess {
		return 0, nil
	}

	if name, ok := statusNames[sc]; ok {
		return sc, fmt.Errorf("admin command 0x%02x failed with NVMe status 0x%03x (%s)", cmd.opcode, sc, name)
	}

	return sc, fmt.Errorf("admin command 0x%02x failed with NVMe status 0x%03x", cmd.opcode, sc)
}

// Reset performs a controller-level reset (NVME_IOCTL_RESET), which discards a
// firmware image that was downloaded but not committed. The kernel quiesces
// the controller's queues, re-initializes it and retries in-flight I/O, so a
// disk in use survives it at the cost of a short stall.
func (d *NVMeDevice) Reset() error {
	if _, errno := d.doIoctl(nvmeIoctlReset, nil, nil); errno != 0 {
		return fmt.Errorf("controller reset failed: %w", errno)
	}

	return nil
}

// DownloadFirmware transfers the image in chunks (Firmware Image Download, 0x11).
//
// The controller keeps a downloaded image staged until a Firmware Commit or a
// controller-level reset, and refuses to overwrite it with 0x114 "overlapping
// range". An image left staged by an earlier attempt (the first chunk fails
// with 0x114) is discarded with a controller reset and the download is retried
// once. A download that fails or is canceled part-way resets the controller
// before returning, so a partial image never stays staged.
func (d *NVMeDevice) DownloadFirmware(ctx context.Context, image []byte) error {
	if len(image) == 0 {
		return errors.New("empty firmware image")
	}

	chunkSize, err := d.chunkSize()
	if err != nil {
		return fmt.Errorf("error determining download chunk size: %w", err)
	}

	sent, status, err := d.download(ctx, image, chunkSize)
	if err == nil {
		return nil
	}

	if sent == 0 && status == statusOverlappingRange {
		sent, err = d.discardStaleImageAndRetry(ctx, image, chunkSize, err)
		if err == nil {
			return nil
		}
	}

	if sent > 0 {
		if resetErr := d.Reset(); resetErr != nil {
			return fmt.Errorf("%w; the controller holds a partial firmware image and %w, reboot before retrying", err, resetErr)
		}
	}

	return err
}

// discardStaleImageAndRetry resets the controller to drop an image staged by
// an earlier, uncommitted download (downloadErr is its 0x114 rejection) and
// runs the download again.
func (d *NVMeDevice) discardStaleImageAndRetry(ctx context.Context, image []byte, chunkSize int, downloadErr error) (int, error) {
	if resetErr := d.Reset(); resetErr != nil {
		return 0, fmt.Errorf("%w; the controller holds a stale firmware image and %w, reboot before retrying", downloadErr, resetErr)
	}

	sent, _, err := d.download(ctx, image, chunkSize)

	return sent, err
}

// download sends the image as chunkSize-byte chunks and reports how many the
// controller accepted; on failure, status is the NVMe status of the rejected
// chunk (0 for an errno failure).
func (d *NVMeDevice) download(ctx context.Context, image []byte, chunkSize int) (int, uint16, error) {
	cmd := adminCmd{
		opcode:    opcodeFirmwareDownload,
		timeoutMS: 120_000,
	}

	sent := 0

	for offset := 0; offset < len(image); offset += chunkSize {
		if err := ctx.Err(); err != nil {
			return sent, 0, fmt.Errorf("firmware download canceled at offset %d: %w", offset, err)
		}

		chunk := firmwareChunk(image, offset, chunkSize)

		cmd.addr = unsafe.Pointer(&chunk[0])
		cmd.dataLen = uint32(len(chunk))
		cmd.cdw10 = uint32(len(chunk)>>2) - 1 // number of dwords, 0-based
		cmd.cdw11 = uint32(offset >> 2)       // offset in dwords

		status, err := d.adminPassthru(&cmd, chunk)
		if err != nil {
			err = fmt.Errorf("error downloading firmware chunk at offset %d: %w", offset, err)

			// an oversized buffer is rejected before it reaches the controller;
			// without a known limit, chunkSize could not catch it.
			if sent == 0 && d.maxTransfer == 0 && errors.Is(err, unix.EINVAL) {
				err = fmt.Errorf("%w (the %d-byte chunk may exceed the kernel transfer limit, which is unknown for a controller without namespaces)", err, len(chunk))
			}

			return sent, status, err
		}

		sent++
	}

	return sent, 0, nil
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

	padded := bytes.Repeat([]byte{0xff}, chunkSize)
	copy(padded, actual)

	return padded
}

// Caps the MDTS exponent before the (1<<mdts) shift so a bogus Identify
// response can't overflow int. 1<<30 * 4 KiB = 4 TiB; real controllers report 5-10.
const mdtsExponentCap = 30

// chunkSize returns the download transfer size honoring FWUG, capped by MDTS
// and by the kernel's transfer limit. Returns an error when a FWUG-aligned
// chunk can't fit under either limit.
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

	if d.maxTransfer != 0 && size > d.maxTransfer {
		return 0, fmt.Errorf("FWUG-aligned chunk size %d exceeds the kernel transfer limit of %d bytes", size, d.maxTransfer)
	}

	return size, nil
}

// identify runs Identify Controller (0x06, CNS=1) and caches MDTS and FWUG.
func (d *NVMeDevice) identify() error {
	data := make([]byte, identifyControllerDataLen)

	cmd := adminCmd{
		opcode:    opcodeIdentify,
		addr:      unsafe.Pointer(&data[0]),
		dataLen:   identifyControllerDataLen,
		cdw10:     cnsIdentifyController,
		timeoutMS: 60_000,
	}

	if _, err := d.adminPassthru(&cmd, data); err != nil {
		return fmt.Errorf("identify controller: %w", err)
	}

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
//
// Commit follows a successful DownloadFirmware. When the commit is canceled or
// rejected, the downloaded image is discarded with a controller reset, as
// DownloadFirmware does on failure. When the command itself fails (errno), the
// controller may or may not have committed the image, so it is left alone.
func (d *NVMeDevice) Commit(ctx context.Context) (Activation, error) {
	if err := ctx.Err(); err != nil {
		return ActivationNone, d.discardImage(fmt.Errorf("firmware commit canceled: %w", err))
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
	case status != statusSuccess:
		return ActivationNone, d.discardImage(err)
	default:
		return ActivationNone, fmt.Errorf("%w; whether the firmware was committed is unknown, reboot before retrying", err)
	}
}

// discardImage resets the controller to drop a downloaded, uncommitted image
// after err, and returns err annotated if the reset fails.
func (d *NVMeDevice) discardImage(err error) error {
	if resetErr := d.Reset(); resetErr != nil {
		return fmt.Errorf("%w; the controller holds an uncommitted firmware image and %w, reboot before retrying", err, resetErr)
	}

	return err
}
