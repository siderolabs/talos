/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package scsi provices a library for working with version 3 SCSI generic
// drivers.
package scsi

import (
	"encoding/hex"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"

	"golang.org/x/sys/unix"
)

const (
	// S represents the SG_IO V3 interface variant identifier.
	S = int32('S')

	// SGIO is the SG_IO ioctl command.
	SGIO = 0x2285
	// SenseBufLen is the length of the sense buffer.
	SenseBufLen = 32
	// SGDxferNone is the SCSI test unit ready command.
	SGDxferNone = -1
	// SGDxferToDev is the SCSI write command.
	SGDxferToDev = -2
	// SGDxferFromDev is the SCSI read command.
	SGDxferFromDev = -3
	// SGGetVersionNum is the SCSI get version command.
	SGGetVersionNum = 0x2282
	// DefaultTimeout is time in milliseconds for SCSI commands.
	DefaultTimeout = 5 * 1000

	// InquiryCmd is the SCSI INQUIRY command.
	InquiryCmd = 0x12
	// InguiryBufLen is the length of the inquiry buffer.
	InguiryBufLen = 252

	// Page0 is VPD page 0.
	Page0 = 0x00
	// Page80 is VPD page 80.
	Page80 = 0x80
	// Page83 is VPD page 83.
	Page83 = 0x83
	// Page83PreSPC3 is VPD page pre-spc3-83.
	Page83PreSPC3 = -0x83

	// VendorLength is the vendor length.
	VendorLength = 8
	// ModelLength is the model length.
	ModelLength = 16

	// MaxSerialLen is the maximum length of the serial number, including
	// added prefixes such as vendor and product (model) strings.
	MaxSerialLen = 256

	// Descriptor types.

	// DescriptorTypeVendorSpecific is the vendor specific descriptor type.
	DescriptorTypeVendorSpecific = 0
	// DescriptorTypeT10Vendor is the t10vendor descriptor type.
	DescriptorTypeT10Vendor = 1
	// DescriptorTypeEUI64 is the eui64 descriptor type.
	DescriptorTypeEUI64 = 2
	// DescriptorTypeNAA is the naa descriptor type.
	DescriptorTypeNAA = 3
	// DescriptorTypeRelport is the relport descriptor type.
	DescriptorTypeRelport = 4
	// DescriptorTypeTgtGroup is the tgtgroup descriptor type.
	DescriptorTypeTgtGroup = 5
	// DescriptorTypeLungroup is the lungroup descriptor type.
	DescriptorTypeLungroup = 6
	// DescriptorTypeMD5 is the md5 descriptor type.
	DescriptorTypeMD5 = 7
	// DescriptorTypeName is the name descriptor type.
	DescriptorTypeName = 8

	// Code set values.

	// CodeSetBinary is the binary code set.
	CodeSetBinary = 1
	// CodeSetASCII is the ASCII code set.
	CodeSetASCII = 2
)

// Type represetns the device type.
type Type int

const (
	// TypeDisk represents the disk device type.
	TypeDisk = iota
	// TypeTape represents the tape device type.
	TypeTape
	// TypeOptical represents the optical device type.
	TypeOptical
	// TypeCD represents the cd device type.
	TypeCD
)

// String returns the string representation of the device type.
func (typ Type) String() string {
	switch typ {
	case TypeDisk:
		return "disk"
	case TypeTape:
		return "tape"
	case TypeOptical:
		return "optical"
	case TypeCD:
		return "cd"
	default:
		return "unknown"
	}
}

// CommandDescriptorBlock represents the command descriptor block.
type CommandDescriptorBlock = [InguiryBufLen]byte

// Sense represents the error reporting buffer.
type Sense = [SenseBufLen]byte

// GenericIOHeader is the control structure for the version 3 SCSI generic
// driver.
type GenericIOHeader struct {
	InterfaceID    int32
	DxferDirection int32
	CmdLen         uint8
	MxSbLen        uint8
	IovecCount     uint16
	DxferLen       uint32
	Dxferp         *byte
	Cmdp           *uint8
	Sbp            *uint8
	Timeout        uint32
	Flags          uint32
	PackID         int32
	pad0           [4]byte // nolint: structcheck
	UsrPtr         *byte
	Status         uint8
	MaskedStatus   uint8
	MsgStatus      uint8
	SbLenWr        uint8
	HostStatus     uint16
	DriverStatus   uint16
	Resid          int32
	Duration       uint32
	Info           uint32
}

// ID represents the set of SCSI device identifiers.
type ID struct {
	Vendor             string
	Model              string
	Revision           string
	Type               Type
	Kernel             string
	Serial             string
	SerialShort        string
	UseSg              int32
	UnitSerialNumber   string
	WWN                string
	WWNVendorExtension string
	TgptGroup          string
}

// Device represents a SCSI device.
type Device struct {
	*ID

	f *os.File
}

func open(name string) (f *os.File, err error) {
	return os.OpenFile(name, os.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
}

// NewDevice returns a SCSI device.
// See https://www.tldp.org/HOWTO/SCSI-Generic-HOWTO/iddriver.html.
func NewDevice(name string) (device *Device, err error) {
	var version uint32
	ptr := uintptr(unsafe.Pointer(&version))

	var f *os.File
	if f, err = open(name); err != nil {
		return nil, err
	}

	if err = ioctl(f.Fd(), SGGetVersionNum, ptr); err != nil {
		return nil, err
	}

	if version < 30000 {
		return nil, errors.Errorf("no sg driver found for device at %s", name)
	}

	device = &Device{
		f: f,
		ID: &ID{
			UseSg: 3,
		},
	}

	return device, nil
}

// Close closes the device.
func (dvc *Device) Close() error {
	return dvc.f.Close()
}

// Identify queries a SCSI device via the SCSI INQUIRY vital product data (VPD)
// page 0x80 and 0x83.
func (dvc *Device) Identify() (err error) {
	// Standard inquiry.
	if err = dvc.StandardInquiry(); err != nil {
		return errors.Errorf("standard inquiry failed: %v", err)
	}

	// Page 0 inquiry.
	var pages []byte
	if pages, err = dvc.Page0Inquiry(); err != nil {
		return errors.Errorf("page 0 inquiry failed: %v", err)
	}
	for _, page := range pages {
		if page == Page80 || page == Page83 {
			// Page 80 inquiry.
			if err = dvc.Page80Inquiry(); err != nil {
				return errors.Errorf("page 80 inquiry failed: %v", err)
			}
		}
		if page == Page83 {
			// Page 83 inquiry.
			if err = dvc.Page83Inquiry(); err != nil {
				return errors.Errorf("page 83 inquiry failed: %v", err)
			}
		}
	}

	return nil
}

// StandardInquiry performs a basic inquiry on the device.
// nolint: gocyclo
func (dvc *Device) StandardInquiry() (err error) {
	var info os.FileInfo
	if info, err = dvc.f.Stat(); err != nil {
		return errors.Errorf("stat failed: %v", err)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("not a syscall.Stat_t")
	}

	dvc.ID.Kernel = fmt.Sprintf("%d:%d", unix.Major(stat.Rdev), unix.Minor(stat.Rdev))

	var buf CommandDescriptorBlock
	if buf, err = dvc.inquiry(0, 0); err != nil {
		return err
	}

	dvc.Vendor = string(buf[8:16])
	dvc.Model = string(buf[16:32])
	dvc.Revision = string(buf[32:36])

	t := buf[1] & 0x1f
	switch t {
	case 0:
		dvc.Type = TypeDisk
	case 1:
		dvc.Type = TypeTape
	case 4:
		dvc.Type = TypeOptical
	case 5:
		dvc.Type = TypeCD
	case 7:
		dvc.Type = TypeOptical
	case 0xe:
		dvc.Type = TypeDisk
	case 0xf:
		dvc.Type = TypeOptical
	default:
	}

	return nil
}

// Page0Inquiry returns a list of Vital Product Data (VPD) pages.
// TODO(andrewrynhard): Should pages by an iota?
func (dvc *Device) Page0Inquiry() (pages []byte, err error) {
	var buf CommandDescriptorBlock

	if buf, err = dvc.inquiry(1, Page0); err != nil {
		return nil, err
	}

	if buf[3] > InguiryBufLen {
		return nil, errors.Errorf("%s: page 0 buffer too big", dvc.Kernel)
	}

	pages = []byte{}
	for i := 4; i < int(buf[3])+3; i++ {
		switch buf[i] {
		case Page80:
			// Note that the fallthrough will append the page.
			fallthrough
		case Page83:
			pages = append(pages, buf[i])
		}
	}

	return pages, nil
}

// Page80Inquiry gets unit serial number VPD page.
func (dvc *Device) Page80Inquiry() (err error) {
	var buf CommandDescriptorBlock

	if buf, err = dvc.inquiry(1, Page80); err != nil {
		return err
	}

	len := 1 + VendorLength + ModelLength + int(buf[3])
	if len > MaxSerialLen {
		return errors.Errorf("%d is larger than %d", len, MaxSerialLen)
	}

	len = int(buf[3])
	dvc.SerialShort = string(buf[4 : len+4])
	dvc.Serial = "S" + dvc.Vendor + dvc.Model + dvc.SerialShort

	return nil
}

// Page83Inquiry sends a page 83 INQUIRY to the device.
func (dvc *Device) Page83Inquiry() (err error) {
	var buf CommandDescriptorBlock

	if buf, err = dvc.inquiry(1, Page83); err != nil {
		return err
	}

	if buf[6] != 0 {
		return errors.New("reply is not SPC-2/3 compliant")
	}

	// Iterate the designation descriptors.
	for i := 4; i <= int(buf[3])+3; i += int(buf[i+3]) + 4 {
		descriptor := buf[i:]
		// Determinine the association.
		// TODO(andrewrynhard): This should be a function.
		if descriptor[1]&0x30 == 0x10 {
		} else if descriptor[1]&0x30 == 0 {
		}

		if descriptor[0]&0x0f == CodeSetASCII {
			continue
		} else {
			dvc.WWN = "0x" + hex.EncodeToString(descriptor[4:4+int(descriptor[3])])
		}
	}

	return nil
}

// evpd: enable vital product data
// page: Vital Product Data (VPD) page
func (dvc *Device) inquiry(evpd, page uint8) (buf CommandDescriptorBlock, err error) {
	buf = CommandDescriptorBlock{}
	sense := Sense{}
	cmdp := []byte{InquiryCmd, evpd, page, 0, InguiryBufLen, 0}

	hdr := &GenericIOHeader{
		InterfaceID:    S,
		Cmdp:           &cmdp[0],
		CmdLen:         uint8(len(cmdp)),
		Dxferp:         &buf[0],
		DxferLen:       InguiryBufLen,
		Sbp:            &sense[0],
		MxSbLen:        SenseBufLen,
		DxferDirection: SGDxferFromDev,
		Timeout:        DefaultTimeout,
	}

	if err = ioctl(dvc.f.Fd(), SGIO, uintptr(unsafe.Pointer(hdr))); err != nil {
		return buf, err
	}

	if hdr.Status != 0 && hdr.HostStatus != 0 && hdr.MsgStatus != 0 && hdr.DriverStatus != 0 {
		return buf, unix.EIO
	}

	if evpd == 1 && buf[1] != page {
		return buf, errors.Errorf("%s: invalid VPD page: %x", dvc.Kernel, page)
	}

	return buf, nil
}

func ioctl(fd, cmd, ptr uintptr) error {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, cmd, ptr)
	if err != 0 {
		return err
	}
	return nil
}
