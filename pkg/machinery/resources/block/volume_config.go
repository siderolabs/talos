// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/redactable"
	"github.com/siderolabs/talos/pkg/machinery/yamlutils"
)

// VolumeConfigType is type of VolumeConfig resource.
const VolumeConfigType = resource.Type("VolumeConfigs.block.talos.dev")

// VolumeConfig resource contains configuration for machine volumes.
type VolumeConfig = typed.Resource[VolumeConfigSpec, VolumeConfigExtension]

// VolumeConfigSpec is the spec for VolumeConfig resource.
//
//gotagsrewrite:gen
//redactgen:gen
type VolumeConfigSpec struct {
	// Parent volume ID, if set no operations on the volume continue until the parent volume is ready.
	ParentID string `yaml:"parentId,omitempty" protobuf:"1"`

	// Volume type.
	Type VolumeType `yaml:"type" protobuf:"2"`

	// Provisioning configuration (how to provision a volume).
	Provisioning ProvisioningSpec `yaml:"provisioning" protobuf:"3"`

	// Encryption configuration (how to encrypt a volume).
	Encryption EncryptionSpec `yaml:"encryption,omitempty" protobuf:"6"`

	// How to find a volume.
	Locator LocatorSpec `yaml:"locator" protobuf:"4"`

	// Mount options for the volume.
	Mount MountSpec `yaml:"mount,omitempty" protobuf:"5"`

	// Symlink options for the volume.
	Symlink SymlinkProvisioningSpec `yaml:"symlink,omitempty" protobuf:"7"`

	// TrimEnabled indicates whether the volume should be trimmed (fstrim) on a schedule.
	TrimEnabled bool `yaml:"trimEnabled,omitempty" protobuf:"8"`
	// TrimInterval is the resolved interval at which the volume should be trimmed.
	TrimInterval time.Duration `yaml:"trimInterval,omitempty" protobuf:"9"`
	// TrimOptions are the resolved options of the trim operation.
	TrimOptions TrimOptionsSpec `yaml:"trimOptions,omitempty" protobuf:"12"`

	// ScrubEnabled indicates whether the volume filesystem should be scrubbed on a schedule.
	ScrubEnabled bool `yaml:"scrubEnabled,omitempty" protobuf:"10"`
	// ScrubInterval is the resolved period at which the volume filesystem should be scrubbed.
	ScrubInterval time.Duration `yaml:"scrubInterval,omitempty" protobuf:"11"`
}

// Wave constants.
const (
	WaveSystemDisk      = -1
	WaveUserVolumes     = 0
	WaveLegacyUserDisks = 1000000 // legacy user disks rely on specific order of provisioning
)

// TrimOptionsSpec is the spec for trim (fstrim) operation options.
//
//gotagsrewrite:gen
type TrimOptionsSpec struct {
	// ChunkSize is the size of the filesystem range trimmed at once (zero means the whole filesystem).
	ChunkSize uint64 `yaml:"chunkSize,omitempty" protobuf:"1"`
	// ChunkDelay is the delay between trimming consecutive chunks.
	ChunkDelay time.Duration `yaml:"chunkDelay,omitempty" protobuf:"2"`
	// MinLength is the minimum contiguous free range to discard.
	MinLength uint64 `yaml:"minLength,omitempty" protobuf:"3"`
}

// ProvisioningSpec is the spec for volume provisioning.
//
//gotagsrewrite:gen
type ProvisioningSpec struct {
	// Provisioning wave for the volume.
	//
	// Waves are processed sequentially - the volumes in the wave are only provisioned after the previous wave is done.
	Wave int `yaml:"wave,omitempty" protobuf:"3"`

	// DiskSelector selects a disk for the volume.
	DiskSelector DiskSelector `yaml:"diskSelector,omitempty" protobuf:"1"`

	// PartitionSpec describes how to provision the volume (partition type).
	PartitionSpec PartitionSpec `yaml:"partitionSpec,omitempty" protobuf:"2"`

	// FilesystemSpec describes how to provision the volume (filesystem type).
	FilesystemSpec FilesystemSpec `yaml:"filesystemSpec,omitempty" protobuf:"4"`
}

// DiskSelector selects a disk for the volume.
//
//gotagsrewrite:gen
type DiskSelector struct {
	Match    cel.Expression `yaml:"match,omitempty" protobuf:"1"`
	External string         `yaml:"external,omitempty" protobuf:"2"`
}

// PartitionSpec is the spec for volume partitioning.
//
//gotagsrewrite:gen
type PartitionSpec struct {
	// Partition minimum size in bytes.
	MinSize uint64 `yaml:"minSize" protobuf:"1"`

	// Partition maximum size in bytes, if not set, grows to the maximum size.
	MaxSize uint64 `yaml:"maxSize,omitempty" protobuf:"2"`

	// Partition maximum size (relative), if not set, grows to the maximum size.
	RelativeMaxSize uint64 `yaml:"relativeMaxSize" protobuf:"6"`

	// NegativeMaxSize indicates that MaxSize or RelativeMaxSize represents space to be left free on the device rather than space to consume.
	NegativeMaxSize bool `yaml:"negativeMaxSize" protobuf:"7"`

	// Grow the partition automatically to the maximum size.
	Grow bool `yaml:"grow" protobuf:"3"`

	// Label for the partition.
	Label string `yaml:"label,omitempty" protobuf:"4"`

	// Partition type UUID.
	TypeUUID string `yaml:"typeUUID,omitempty" protobuf:"5"`
}

// ResolveMaxSize resolves the maximum size of the partition.
// Returns the maximum size of the partition and an error if the partition size cannot be resolved.
func (ps *PartitionSpec) ResolveMaxSize(available uint64) (uint64, error) {
	var size uint64

	if ps.RelativeMaxSize != 0 {
		size = available * ps.RelativeMaxSize / 100
	} else {
		size = ps.MaxSize
	}

	if ps.NegativeMaxSize {
		if size > available {
			return 0, fmt.Errorf("partition size cannot be negative")
		}

		return available - size, nil
	}

	return size, nil
}

// LocatorSpec is the spec for volume locator.
//
//gotagsrewrite:gen
type LocatorSpec struct {
	// Match is a volume locator match expression.
	Match cel.Expression `yaml:"match,omitempty" protobuf:"1"`
	// DiskMatch is a disk locator match expression.
	DiskMatch cel.Expression `yaml:"diskMatch,omitempty" protobuf:"2"`
}

// FilesystemSpec is the spec for volume filesystem.
//
//gotagsrewrite:gen
type FilesystemSpec struct {
	// Filesystem type.
	Type FilesystemType `yaml:"type" protobuf:"1"`
	// Filesystem label.
	Label string `yaml:"label,omitempty" protobuf:"2"`
	// MinAllocationGroupSize is the minimum XFS allocation group size in bytes.
	//
	// Zero leaves the mkfs.xfs defaults alone.
	MinAllocationGroupSize uint64 `yaml:"minAllocationGroupSize,omitempty" protobuf:"3"`
}

// EncryptionSpec is the spec for volume encryption.
//
//gotagsrewrite:gen
type EncryptionSpec struct {
	Provider      EncryptionProviderType `yaml:"provider" protobuf:"1"`
	Keys          []EncryptionKey        `yaml:"keys" protobuf:"2"`
	Cipher        string                 `yaml:"cipher,omitempty" protobuf:"3"`
	KeySize       uint                   `yaml:"keySize,omitempty" protobuf:"4"`
	BlockSize     uint64                 `yaml:"blockSize,omitempty" protobuf:"5"`
	PerfOptions   []string               `yaml:"perfOptions,omitempty" protobuf:"6"`
	AllowDiscards bool                   `yaml:"allowDiscards,omitempty" protobuf:"7"`
}

// EncryptionKey is the spec for volume encryption key.
//
//gotagsrewrite:gen
type EncryptionKey struct {
	Slot        int               `yaml:"slot" protobuf:"1"`
	Type        EncryptionKeyType `yaml:"type" protobuf:"2"`
	LockToSTATE bool              `yaml:"lockToState,omitempty" protobuf:"6"`

	// Only for Type == "static":
	StaticPassphrase yamlutils.StringBytes `yaml:"staticPassphrase,omitempty" protobuf:"3" redact:"replace"`

	// Only for Type == "kms":
	KMSEndpoint string `yaml:"kmsEndpoint,omitempty" protobuf:"4"`

	// Only for Type == "tpm":
	TPMCheckSecurebootStatusOnEnroll bool `yaml:"tpmCheckSecurebootStatusOnEnroll,omitempty" protobuf:"5"`
	// Only for Type == "tpm":
	TPMPCRs []int `yaml:"pcrs,omitempty" protobuf:"7"`
	// Only for Type == "tpm":
	TPMPubKeyPCRs []int `yaml:"pubKeyPcrs,omitempty" protobuf:"8"`
}

// ParameterSpec is a mount parameter.
//
//gotagsrewrite:gen
type ParameterSpec struct {
	// Type of the parameter.
	Type FSParameterType `yaml:"type" protobuf:"1"`
	// Name of the parameter.
	Name string `yaml:"name" protobuf:"2"`
	// String value of the parameter.
	String *string `yaml:"string,omitempty" protobuf:"3"`
	// Binary value of the parameter.
	Binary []byte `yaml:"binary,omitempty" protobuf:"5"`
}

// TmpfsSizeParameter is the mount parameter bounding the size of a tmpfs volume.
//
// The kernel accepts a size in bytes, with an optional k/m/g/t/p/e suffix, or a percentage of the memory.
const TmpfsSizeParameter = "size"

// tmpfsRootParameters set the owner and the permissions of the tmpfs root.
//
// The mount spec (FileMode, UID, GID) is applied to the root after mounting and would override them,
// so they are rejected in favor of it.
var tmpfsRootParameters = []string{"mode", "uid", "gid"}

// tmpfsBlocksParameter sets the same limit as the size parameter, in pages.
//
// The kernel applies whichever of the two comes last, and "nr_blocks=0" makes the tmpfs unbounded,
// so it is rejected in favor of the size parameter.
const tmpfsBlocksParameter = "nr_blocks"

// ValidateTmpfsParameters checks the mount parameters of a tmpfs volume.
func ValidateTmpfsParameters(parameters []ParameterSpec) error {
	var hasSize bool

	for _, param := range parameters {
		// A parameter without a value is skipped on mount, so it would be silently ignored.
		if !param.HasValue() {
			return fmt.Errorf("tmpfs parameter %q has no value", param.Name)
		}

		if param.Name == TmpfsSizeParameter && param.Type == FSParameterTypeStringValue {
			if _, _, err := parseTmpfsSize(*param.String); err != nil {
				return fmt.Errorf("invalid tmpfs size %q: %w", *param.String, err)
			}

			// The kernel applies the last size, so another one is most likely a mistake.
			if hasSize {
				return fmt.Errorf("tmpfs parameter %q is set more than once", TmpfsSizeParameter)
			}

			hasSize = true
		}

		if param.Name == tmpfsBlocksParameter {
			return fmt.Errorf("tmpfs parameter %q is not supported, use %q instead", param.Name, TmpfsSizeParameter)
		}

		if slices.Contains(tmpfsRootParameters, param.Name) {
			return fmt.Errorf("tmpfs parameter %q is not supported, use the mount file mode, UID and GID instead", param.Name)
		}
	}

	// An unbounded tmpfs may grow up to half of the memory.
	if !hasSize {
		return fmt.Errorf("tmpfs requires the %q parameter", TmpfsSizeParameter)
	}

	return nil
}

// TmpfsSize returns the size of a tmpfs volume in bytes, if its size parameter is set in bytes.
//
// The size is resolved the way the kernel does: the last size parameter wins, a suffix is converted to bytes,
// and the size is rounded up to a whole page. A percentage of the memory is not resolved, and an invalid size,
// including zero (an unbounded tmpfs), is not reported.
func TmpfsSize(parameters []ParameterSpec, pageSize uint64) (uint64, bool) {
	value := lastTmpfsSize(parameters)
	if value == nil {
		return 0, false
	}

	size, percent, err := parseTmpfsSize(*value)
	if err != nil || percent {
		return 0, false
	}

	return roundUpToPage(size, pageSize)
}

// lastTmpfsSize returns the value of the last size parameter, which is the one the kernel applies.
func lastTmpfsSize(parameters []ParameterSpec) *string {
	var value *string

	for _, param := range parameters {
		if param.Name == TmpfsSizeParameter && param.Type == FSParameterTypeStringValue && param.String != nil {
			value = param.String
		}
	}

	return value
}

// roundUpToPage rounds the size up to a whole page, reporting an overflow.
func roundUpToPage(size, pageSize uint64) (uint64, bool) {
	if pageSize == 0 {
		return 0, false
	}

	pages := size / pageSize
	if size%pageSize != 0 {
		pages++
	}

	if pages > math.MaxUint64/pageSize {
		return 0, false
	}

	return pages * pageSize, true
}

// parseTmpfsSize parses the tmpfs size parameter like the kernel does (memparse, then an optional percent sign),
// but more strictly.
//
// The number is in base 0: a "0x" prefix selects hexadecimal and a leading zero octal. It may be followed by a
// k/m/g/t/p/e suffix, or by a percent sign for a percentage of the memory. Unlike the kernel:
//   - an overflow is an error, while the kernel silently wraps around;
//   - a zero size is an error, while the kernel treats it as an unbounded tmpfs;
//   - a suffix followed by a percent sign ("1k%", 1024% of the memory) is an error, while the kernel accepts it.
func parseTmpfsSize(value string) (size uint64, percent bool, err error) {
	size, rest, err := parseBase0(value)
	if err != nil {
		return 0, false, err
	}

	shift, percent, err := parseTmpfsSizeSuffix(rest)
	if err != nil {
		return 0, false, err
	}

	if size > math.MaxUint64>>shift {
		return 0, false, errors.New("size out of range")
	}

	if size == 0 {
		return 0, false, errors.New("size 0 makes the tmpfs unbounded")
	}

	return size << shift, percent, nil
}

// parseBase0 parses the leading number of value the way the kernel simple_strtoull does with base 0,
// returning the rest of value. Unlike the kernel, an overflow is an error.
func parseBase0(value string) (uint64, string, error) {
	base, digits := detectBase(value)

	end := 0
	for end < len(digits) && digitValue(digits[end]) < base {
		end++
	}

	if end == 0 {
		return 0, "", errors.New("expected a number")
	}

	n, err := strconv.ParseUint(digits[:end], base, 64)
	if err != nil {
		return 0, "", errors.New("size out of range")
	}

	return n, digits[end:], nil
}

// detectBase returns the base of a number in base 0 and its digits: a "0x" prefix followed by a hexadecimal digit
// selects hexadecimal, and a leading zero octal (the zero is kept, as an octal digit).
func detectBase(value string) (int, string) {
	switch {
	case len(value) > 2 && value[0] == '0' && value[1]|0x20 == 'x' && digitValue(value[2]) < 16:
		return 16, value[2:]
	case len(value) > 1 && value[0] == '0':
		return 8, value
	default:
		return 10, value
	}
}

// parseTmpfsSizeSuffix parses what follows the number of a tmpfs size: an optional k/m/g/t/p/e suffix,
// or a percent sign.
func parseTmpfsSizeSuffix(rest string) (shift uint, percent bool, err error) {
	if rest != "" {
		if i := strings.IndexByte("kmgtpe", rest[0]|0x20); i >= 0 {
			shift = 10 * uint(i+1)
			rest = rest[1:]
		}
	}

	if rest == "%" && shift == 0 {
		return 0, true, nil
	}

	if rest != "" {
		return 0, false, fmt.Errorf("unexpected %q, expected an optional k/m/g/t/p/e suffix or a percent sign", rest)
	}

	return shift, false, nil
}

// digitValue returns the value of a hexadecimal digit, or 16 if c is not one.
func digitValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c|0x20 >= 'a' && c|0x20 <= 'f':
		return int(c|0x20-'a') + 10
	default:
		return 16
	}
}

// HasValue reports whether the parameter is applied on mount.
//
// A boolean parameter has no value of its own, while a string or binary parameter without a value is skipped.
func (p ParameterSpec) HasValue() bool {
	switch p.Type {
	case FSParameterTypeBooleanValue:
		return true
	case FSParameterTypeStringValue:
		return p.String != nil
	case FSParameterTypeBinaryValue:
		return p.Binary != nil
	default:
		return false
	}
}

// NewBooleanParameter creates a new boolean parameter.
func NewBooleanParameter(name string) ParameterSpec {
	return ParameterSpec{
		Type: FSParameterTypeBooleanValue,
		Name: name,
	}
}

// NewBinaryParameter creates a new binary parameter.
func NewBinaryParameter(name string, value []byte) ParameterSpec {
	return ParameterSpec{
		Type:   FSParameterTypeBinaryValue,
		Name:   name,
		Binary: value,
	}
}

// NewStringParameter creates a new string parameter.
func NewStringParameter(name, value string) ParameterSpec {
	return ParameterSpec{
		Type:   FSParameterTypeStringValue,
		Name:   name,
		String: new(value),
	}
}

// MountSpec is the spec for volume mount.
//
//gotagsrewrite:gen
type MountSpec struct {
	// Mount path for the volume.
	TargetPath string `yaml:"targetPath" protobuf:"1"`
	// SELinux label for the volume.
	SelinuxLabel string `yaml:"selinuxLabel" protobuf:"2"`
	// Enable project quota (xfs) for the volume.
	ProjectQuotaSupport bool `yaml:"projectQuotaSupport" protobuf:"3"`
	// Parent mount request ID.
	ParentID string `yaml:"parentId,omitempty" protobuf:"4"`
	// FileMode is the file mode for the mount target.
	FileMode os.FileMode `yaml:"fileMode,omitempty" protobuf:"5"`
	// UID is the user ID for the mount target.
	UID int `yaml:"uid,omitempty" protobuf:"6"`
	// GID is the group ID for the mount target.
	GID int `yaml:"gid,omitempty" protobuf:"7"`
	// RecursiveRelabel is the recursive relabel/chown flag for the mount target.
	RecursiveRelabel bool `yaml:"recursiveRelabel,omitempty" protobuf:"8"`
	// BindTarget is an optional path on the host to bind-mount the volume onto.
	BindTarget *string `yaml:"bindTarget,omitempty" protobuf:"9"`
	// Parameters are additional filesystem mount options used when mounting the volume.
	Parameters []ParameterSpec `yaml:"parameters,omitempty" protobuf:"10"`
	// Secure applies MOUNT_ATTR_NOSUID\|NODEV to the mount.
	Secure bool `yaml:"secure,omitempty" protobuf:"11"`
	// NoExec applies MOUNT_ATTR_NOEXEC to the mount.
	NoExec bool `yaml:"noExec,omitempty" protobuf:"12"`
}

// SymlinkProvisioningSpec is the spec for volume symlink.
//
//gotagsrewrite:gen
type SymlinkProvisioningSpec struct {
	// Symlink target path for the volume.
	SymlinkTargetPath string `yaml:"symlinkTargetPath" protobuf:"1"`
	// Force symlink creation.
	Force bool `yaml:"force" protobuf:"2"`
}

// NewVolumeConfig initializes a BlockVolumeConfig resource.
func NewVolumeConfig(namespace resource.Namespace, id resource.ID) *VolumeConfig {
	return typed.NewResource[VolumeConfigSpec, VolumeConfigExtension](
		resource.NewMetadata(namespace, VolumeConfigType, id, resource.VersionUndefined),
		VolumeConfigSpec{},
	)
}

// VolumeConfigExtension is auxiliary resource data for BlockVolumeConfig.
type VolumeConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VolumeConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VolumeConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(VolumeConfigType, &VolumeConfig{})
	if err != nil {
		panic(err)
	}

	// the spec is sensitive, but it is safe to be read in the redacted form
	redactable.Register[VolumeConfigSpec, VolumeConfigExtension]()
}
