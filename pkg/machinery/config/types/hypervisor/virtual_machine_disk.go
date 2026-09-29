// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// maxFileNameLength bounds a content library file name at NAME_MAX.
//
// The API that writes into a library bounds it more tightly, to leave room for the name an upload
// is staged under; that bound is not visible from here. See validateImageFileName.
const maxFileNameLength = 255

// supportedDigestAlgorithms are the algorithms a content library hashes a file under, and so the
// only ones a digest pinned on an image reference can be checked against.
var supportedDigestAlgorithms = []digest.Algorithm{digest.SHA256, digest.SHA512}

// Check interfaces.
var (
	_ config.VirtualMachineDiskConfig          = &VirtualMachineDisk{}
	_ config.VirtualMachineDiskProvisionConfig = &VirtualMachineDiskProvision{}
	_ config.VirtualMachineDiskFromImageConfig = &VirtualMachineDiskFromImage{}
)

// VirtualMachineDiskList is a list of VirtualMachineDisks with an overridden merge process.
//
//docgen:alias
type VirtualMachineDiskList []VirtualMachineDisk

// Merge the disks by name.
func (list *VirtualMachineDiskList) Merge(other any) error {
	otherDisks, ok := other.(VirtualMachineDiskList)
	if !ok {
		return fmt.Errorf("unexpected type for disk merge %T", other)
	}

	for _, disk := range otherDisks {
		if err := list.mergeDisk(disk); err != nil {
			return err
		}
	}

	return nil
}

func (list *VirtualMachineDiskList) mergeDisk(disk VirtualMachineDisk) error {
	var existing *VirtualMachineDisk

	if disk.DiskName != "" {
		for idx := range *list {
			if (*list)[idx].DiskName == disk.DiskName {
				existing = &(*list)[idx]

				break
			}
		}
	}

	if existing != nil {
		return merge.Merge(existing, &disk)
	}

	*list = append(*list, disk)

	return nil
}

// VirtualMachineDisk describes a single disk attached to a virtual machine.
type VirtualMachineDisk struct {
	//   description: |
	//     Name of the disk, unique within the virtual machine.
	//
	//     Must be between 1 and 63 characters long, and can only contain ASCII letters,
	//     digits and hyphens. It names the volume created in the storage pool.
	//   schemaRequired: true
	DiskName string `yaml:"name"`
	//   description: |
	//     Name of the `StoragePoolConfig` document this disk's volume lives in.
	//
	//     The pool is declared separately and is not provisioned by this document. The reference
	//     is checked for shape only: nothing resolves it against the rest of the machine
	//     configuration yet.
	//   examples:
	//     - value: '"pool1"'
	//   schemaRequired: true
	DiskPool string `yaml:"pool"`
	//   description: |
	//     Size of the volume.
	//
	//     Size is specified in bytes, but can be expressed in human readable format, e.g. 20GiB.
	//
	//     Required for a `disk`, and not allowed on a `cdrom`, whose size is that of its image.
	//   schema:
	//     type: string
	DiskSize meta.ByteSize `yaml:"size,omitempty"`
	//   description: |
	//     On-disk format of the volume.
	//
	//     This is not cosmetic: `provision.fromImage.mode: linked` requires `qcow2`, since backing
	//     chains are a qcow2 feature, while `raw` is faster on block-backed pools.
	//
	//     Optional; defaults to `qcow2`. Not allowed on a `cdrom`, which is used as-is.
	//   values:
	//     - raw
	//     - qcow2
	DiskFormat hypervisorhelpers.VirtualMachineDiskFormat `yaml:"format,omitempty"`
	//   description: |
	//     Controller the disk is attached to.
	//
	//     `virtio` for anything modern; `sata` for guests without virtio drivers at install time.
	//
	//     Optional; defaults to `virtio` on a `disk` and to `sata` on a `cdrom`. A `cdrom` cannot
	//     be attached to `virtio`, which presents no ejectable media.
	//   values:
	//     - virtio
	//     - scsi
	//     - sata
	//     - nvme
	DiskBus hypervisorhelpers.VirtualMachineDiskBus `yaml:"bus,omitempty"`
	//   description: |
	//     Kind of device the disk is presented as.
	//
	//     A `cdrom` is read-only -- QEMU emulates no CD burner -- so its contents are required and
	//     it has no size and no format of its own.
	//
	//     Optional; defaults to `disk`.
	//   values:
	//     - disk
	//     - cdrom
	DiskType hypervisorhelpers.VirtualMachineDiskType `yaml:"type,omitempty"`
	//   description: |
	//     Position of this disk in the guest's boot order, lowest first.
	//
	//     Values must be unique across everything the virtual machine can boot from. Only disks
	//     are bootable today, so that is only the disks; network interfaces will share this
	//     namespace once they are configurable.
	//
	//     Optional; a disk without a boot order is not booted from.
	DiskBootOrder uint32 `yaml:"bootOrder,omitempty"`
	//   description: |
	//     Where the volume's contents come from.
	//
	//     Exactly one source must be set.
	//   schemaRequired: true
	ProvisionConfig VirtualMachineDiskProvision `yaml:"provision"`
}

// VirtualMachineDiskProvision describes where a disk's contents come from.
//
// Exactly one source must be set.
type VirtualMachineDiskProvision struct {
	//   description: |
	//     Create an empty volume, formatted per `format`.
	//
	//     Not allowed on a `cdrom`, which has no meaningful empty contents.
	BlankConfig *VirtualMachineDiskBlank `yaml:"blank,omitempty"`
	//   description: |
	//     Derive the volume from an image held in a content library.
	FromImageConfig *VirtualMachineDiskFromImage `yaml:"fromImage,omitempty"`
}

// VirtualMachineDiskBlank provisions an empty volume.
//
// It carries no settings: the volume's size and format are the disk's own.
type VirtualMachineDiskBlank struct{}

// VirtualMachineDiskFromImage derives a volume from a content library image.
type VirtualMachineDiskFromImage struct {
	//   description: |
	//     Name of the `ContentLibraryConfig` document holding the image.
	//   examples:
	//     - value: '"images"'
	//   schemaRequired: true
	ImageLibrary string `yaml:"library"`
	//   description: |
	//     Name of the file within that library.
	//   examples:
	//     - value: '"talos-1.14.qcow2"'
	//   schemaRequired: true
	ImageFile string `yaml:"file"`
	//   description: |
	//     Integrity check of the library file, verified before the volume is provisioned.
	//
	//     Written as `<algorithm>:<hex>`, under either `sha256` or `sha512`.
	//
	//     Optional; the file is used as-is when this is unset.
	//   examples:
	//     - value: '"sha256:5f2bc19e8b4b5b4a8b5e9c0d1f2a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c"'
	ImageDigest string `yaml:"digest,omitempty"`
	//   description: |
	//     How the volume is derived from the image.
	//
	//     `copy` makes a full, independent copy. `linked` makes a thin qcow2 backed by the library
	//     image: fast and space-cheap, but it pins that image for the lifetime of the disk, and it
	//     requires `format: qcow2`.
	//
	//     Optional; defaults to `copy`.
	//   values:
	//     - copy
	//     - linked
	ImageMode hypervisorhelpers.VirtualMachineDiskImageMode `yaml:"mode,omitempty"`
}

// Name implements config.VirtualMachineDiskConfig interface.
func (d *VirtualMachineDisk) Name() string {
	return d.DiskName
}

// Pool implements config.VirtualMachineDiskConfig interface.
func (d *VirtualMachineDisk) Pool() string {
	return d.DiskPool
}

// Size implements config.VirtualMachineDiskConfig interface.
func (d *VirtualMachineDisk) Size() uint64 {
	return d.DiskSize.Value()
}

// Format implements config.VirtualMachineDiskConfig interface.
func (d *VirtualMachineDisk) Format() hypervisorhelpers.VirtualMachineDiskFormat {
	if d.DiskFormat == hypervisorhelpers.VirtualMachineDiskFormatUnknown {
		return hypervisorhelpers.VirtualMachineDiskFormatQCOW2
	}

	return d.DiskFormat
}

// Bus implements config.VirtualMachineDiskConfig interface.
//
// A cdrom defaults to SATA rather than virtio: virtio-blk presents no ejectable media, so
// libvirt rejects the combination with "does not support ejectable media".
func (d *VirtualMachineDisk) Bus() hypervisorhelpers.VirtualMachineDiskBus {
	if d.DiskBus != hypervisorhelpers.VirtualMachineDiskBusUnknown {
		return d.DiskBus
	}

	if d.Type() == hypervisorhelpers.VirtualMachineDiskTypeCDROM {
		return hypervisorhelpers.VirtualMachineDiskBusSATA
	}

	return hypervisorhelpers.VirtualMachineDiskBusVirtio
}

// Type implements config.VirtualMachineDiskConfig interface.
func (d *VirtualMachineDisk) Type() hypervisorhelpers.VirtualMachineDiskType {
	if d.DiskType == hypervisorhelpers.VirtualMachineDiskTypeUnknown {
		return hypervisorhelpers.VirtualMachineDiskTypeDisk
	}

	return d.DiskType
}

// BootOrder implements config.VirtualMachineDiskConfig interface.
func (d *VirtualMachineDisk) BootOrder() uint32 {
	return d.DiskBootOrder
}

// Provision implements config.VirtualMachineDiskConfig interface.
func (d *VirtualMachineDisk) Provision() config.VirtualMachineDiskProvisionConfig {
	return &d.ProvisionConfig
}

// Blank implements config.VirtualMachineDiskProvisionConfig interface.
func (p *VirtualMachineDiskProvision) Blank() bool {
	return p.BlankConfig != nil
}

// FromImage implements config.VirtualMachineDiskProvisionConfig interface.
func (p *VirtualMachineDiskProvision) FromImage() optional.Optional[config.VirtualMachineDiskFromImageConfig] {
	if p.FromImageConfig == nil {
		return optional.None[config.VirtualMachineDiskFromImageConfig]()
	}

	return optional.Some[config.VirtualMachineDiskFromImageConfig](p.FromImageConfig)
}

// Library implements config.VirtualMachineDiskFromImageConfig interface.
func (i *VirtualMachineDiskFromImage) Library() string {
	return i.ImageLibrary
}

// File implements config.VirtualMachineDiskFromImageConfig interface.
func (i *VirtualMachineDiskFromImage) File() string {
	return i.ImageFile
}

// Digest implements config.VirtualMachineDiskFromImageConfig interface.
func (i *VirtualMachineDiskFromImage) Digest() string {
	return i.ImageDigest
}

// Mode implements config.VirtualMachineDiskFromImageConfig interface.
func (i *VirtualMachineDiskFromImage) Mode() hypervisorhelpers.VirtualMachineDiskImageMode {
	if i.ImageMode == hypervisorhelpers.VirtualMachineDiskImageModeUnknown {
		return hypervisorhelpers.VirtualMachineDiskImageModeCopy
	}

	return i.ImageMode
}

// Validate checks the disk and returns its name.
//
//nolint:gocyclo,cyclop
func (d *VirtualMachineDisk) Validate(index int) (string, error) {
	var validationErrors error

	if err := validateName(d.DiskName); err != nil {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("disks[%d]: %w", index, err))
	}

	switch {
	case d.DiskPool == "":
		validationErrors = errors.Join(validationErrors, fmt.Errorf("disks[%d]: pool is required", index))
	case !validNamePattern.MatchString(d.DiskPool):
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("disks[%d]: pool %q: pool name can only contain ASCII letters, digits and hyphens", index, d.DiskPool))
	}

	//nolint:exhaustive // Type() resolves the zero member to disk, so it never reaches this switch.
	switch d.Type() {
	case hypervisorhelpers.VirtualMachineDiskTypeDisk:
		switch {
		case d.DiskSize.IsNegative():
			validationErrors = errors.Join(validationErrors, fmt.Errorf("disks[%d]: size must not be negative", index))
		case d.DiskSize.IsZero():
			validationErrors = errors.Join(validationErrors, fmt.Errorf("disks[%d]: size is required", index))
		case d.DiskSize.Value() == 0:
			validationErrors = errors.Join(validationErrors, fmt.Errorf("disks[%d]: size must be greater than zero", index))
		}
	case hypervisorhelpers.VirtualMachineDiskTypeCDROM:
		if !d.DiskSize.IsZero() {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("disks[%d]: size is not allowed on a cdrom", index))
		}

		if d.DiskFormat != hypervisorhelpers.VirtualMachineDiskFormatUnknown {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("disks[%d]: format is not allowed on a cdrom", index))
		}

		if d.DiskBus == hypervisorhelpers.VirtualMachineDiskBusVirtio {
			validationErrors = errors.Join(validationErrors,
				fmt.Errorf("disks[%d]: bus virtio is not allowed on a cdrom, which presents ejectable media", index))
		}
	default:
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("disks[%d]: unsupported type %q, expected %s", index, d.DiskType,
				expectedValues(hypervisorhelpers.VirtualMachineDiskTypeStrings())))
	}

	if d.DiskFormat != hypervisorhelpers.VirtualMachineDiskFormatUnknown && !d.DiskFormat.IsAVirtualMachineDiskFormat() {
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("disks[%d]: unsupported format %q, expected %s", index, d.DiskFormat,
				expectedValues(hypervisorhelpers.VirtualMachineDiskFormatStrings())))
	}

	if d.DiskBus != hypervisorhelpers.VirtualMachineDiskBusUnknown && !d.DiskBus.IsAVirtualMachineDiskBus() {
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("disks[%d]: unsupported bus %q, expected %s", index, d.DiskBus,
				expectedValues(hypervisorhelpers.VirtualMachineDiskBusStrings())))
	}

	validationErrors = errors.Join(validationErrors, d.validateProvision(index))

	return d.DiskName, validationErrors
}

// validateProvision checks the disk's provisioning source.
func (d *VirtualMachineDisk) validateProvision(index int) error {
	switch {
	case d.ProvisionConfig.BlankConfig != nil && d.ProvisionConfig.FromImageConfig != nil:
		return fmt.Errorf("disks[%d]: provision: blank and fromImage are mutually exclusive", index)
	case d.ProvisionConfig.BlankConfig == nil && d.ProvisionConfig.FromImageConfig == nil:
		return fmt.Errorf("disks[%d]: provision: exactly one of blank or fromImage must be set", index)
	case d.ProvisionConfig.BlankConfig != nil:
		if d.Type() == hypervisorhelpers.VirtualMachineDiskTypeCDROM {
			return fmt.Errorf("disks[%d]: provision.blank: a cdrom has no contents of its own", index)
		}

		return nil
	}

	return d.ProvisionConfig.FromImageConfig.validate(index, d.DiskFormat, d.Type())
}

// validate checks the image reference.
//
// diskFormat is the disk's format as written, not as defaulted: `linked` needs a qcow2 volume, and
// the default is already qcow2, so only an explicit `raw` conflicts. diskType is the disk's
// defaulted type, used to reject `linked` on a cdrom, which has no backing chain of its own.
func (i *VirtualMachineDiskFromImage) validate(
	index int,
	diskFormat hypervisorhelpers.VirtualMachineDiskFormat,
	diskType hypervisorhelpers.VirtualMachineDiskType,
) error {
	var validationErrors error

	switch {
	case i.ImageLibrary == "":
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("disks[%d]: provision.fromImage.library is required", index))
	case !validNamePattern.MatchString(i.ImageLibrary):
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("disks[%d]: provision.fromImage.library %q: library name can only contain ASCII letters, digits and hyphens",
				index, i.ImageLibrary))
	}

	validationErrors = errors.Join(validationErrors, validateImageFileName(index, i.ImageFile))

	if i.ImageDigest != "" {
		validationErrors = errors.Join(validationErrors, validateImageDigest(index, i.ImageDigest))
	}

	switch i.ImageMode {
	case hypervisorhelpers.VirtualMachineDiskImageModeUnknown, hypervisorhelpers.VirtualMachineDiskImageModeCopy:
	case hypervisorhelpers.VirtualMachineDiskImageModeLinked:
		switch {
		case diskType == hypervisorhelpers.VirtualMachineDiskTypeCDROM:
			validationErrors = errors.Join(validationErrors,
				fmt.Errorf("disks[%d]: provision.fromImage.mode: linked is not allowed on a cdrom, which has no backing chain of its own", index))
		case diskFormat == hypervisorhelpers.VirtualMachineDiskFormatRaw:
			validationErrors = errors.Join(validationErrors,
				fmt.Errorf("disks[%d]: provision.fromImage.mode: linked requires format qcow2, as backing chains are a qcow2 feature", index))
		}
	default:
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("disks[%d]: unsupported provision.fromImage.mode %q, expected %s", index, i.ImageMode,
				expectedValues(hypervisorhelpers.VirtualMachineDiskImageModeStrings())))
	}

	return validationErrors
}

// validateImageFileName checks a file name within a content library.
//
// Only the forms which can never name a file are rejected here. The authoritative check lives with
// the API that writes into a library, in internal/app/contentlibrary: its length bound leaves room
// for the name an upload is staged under, which machinery cannot see.
func validateImageFileName(index int, name string) error {
	switch {
	case name == "":
		return fmt.Errorf("disks[%d]: provision.fromImage.file is required", index)
	case len(name) > maxFileNameLength:
		return fmt.Errorf("disks[%d]: provision.fromImage.file %q must be %d characters or fewer", index, name, maxFileNameLength)
	case strings.ContainsRune(name, os.PathSeparator):
		return fmt.Errorf("disks[%d]: provision.fromImage.file %q must not contain a path separator", index, name)
	case strings.HasPrefix(name, "."):
		return fmt.Errorf("disks[%d]: provision.fromImage.file %q must not start with a dot", index, name)
	}

	return nil
}

// validateImageDigest checks the digest pinned on an image reference.
//
// go-digest registers algorithms the node does not hash a library file under, so a well-formed
// digest naming one of those is refused here rather than accepted and then left unverified.
func validateImageDigest(index int, s string) error {
	dgst, err := digest.Parse(s)
	if err != nil {
		return fmt.Errorf("disks[%d]: provision.fromImage.digest %q is invalid: %w", index, s, err)
	}

	if !slices.Contains(supportedDigestAlgorithms, dgst.Algorithm()) {
		return fmt.Errorf("disks[%d]: provision.fromImage.digest algorithm %q is not supported, expected one of %v",
			index, dgst.Algorithm(), supportedDigestAlgorithms)
	}

	return nil
}
