// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-pointer"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// UserVolumeConfigKind is a config document kind.
const UserVolumeConfigKind = "UserVolumeConfig"

func init() {
	registry.Register(UserVolumeConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &UserVolumeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.UserVolumeConfig    = &UserVolumeConfigV1Alpha1{}
	_ config.ConflictingDocument = &UserVolumeConfigV1Alpha1{}
	_ config.NamedDocument       = &UserVolumeConfigV1Alpha1{}
	_ config.Validator           = &UserVolumeConfigV1Alpha1{}
	_ config.SecretDocument      = &UserVolumeConfigV1Alpha1{}
)

// tmpfsWarnPercent is the size of a tmpfs volume, in percents of the memory, above which a warning is reported.
//
// It matches the size of a tmpfs mounted without a size.
const tmpfsWarnPercent = 50

// VolumeType is an alias for block.VolumeType.
type VolumeType = block.VolumeType

// UserVolumeConfigV1Alpha1 is a user volume configuration document.
//
//	description: |
//	  User volume is automatically allocated as a partition on the specified disk
//	  and mounted under `/var/mnt/<name>`.
//	  The partition label is automatically generated as `u-<name>`.
//	  The name must not be taken by an existing, external or inline data volume.
//	  A `tmpfs` user volume is backed by memory, its size is set with `provisioning.maxSize`
//	  (in bytes, or in percents of the memory), and its content is lost on unmount.
//	examples:
//	  - value: exampleUserVolumeConfigV1Alpha1Directory()
//	  - value: exampleUserVolumeConfigV1Alpha1Disk()
//	  - value: exampleUserVolumeConfigV1Alpha1Partition()
//	  - value: exampleUserVolumeConfigV1Alpha1Tmpfs()
//	  - value: exampleUserVolumeConfigV1Alpha1Owned()
//	alias: UserVolumeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/UserVolumeConfig
type UserVolumeConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the volume.
	//
	//     Name might be between 1 and 34 characters long and can only contain:
	//     lowercase and uppercase ASCII letters, digits, and hyphens.
	MetaName string `yaml:"name"`
	//   description: |
	//     Volume type.
	//   values:
	//     - directory
	//     - disk
	//     - partition
	//     - tmpfs
	VolumeType *VolumeType `yaml:"volumeType,omitempty"`
	//   description: |
	//     The provisioning describes how the volume is provisioned.
	ProvisioningSpec ProvisioningSpec `yaml:"provisioning,omitempty"`
	//   description: |
	//     The filesystem describes how the volume is formatted.
	FilesystemSpec FilesystemSpec `yaml:"filesystem,omitempty"`
	//   description: |
	//     The encryption describes how the volume is encrypted.
	EncryptionSpec EncryptionSpec `yaml:"encryption,omitempty"`
	//   description: |
	//     The mount describes additional mount options.
	MountSpec UserMountSpec `yaml:"mount,omitempty"`
	//   description: |
	//     The trim describes the per-volume filesystem trim (fstrim) configuration.
	TrimSpec *TrimConfig `yaml:"trim,omitempty"`
	//   description: |
	//     The scrub describes the per-volume filesystem scrub configuration.
	ScrubSpec *ScrubConfig `yaml:"scrub,omitempty"`
}

// UserMountSpec describes how the volume is mounted.
type UserMountSpec struct {
	//   description: |
	//     If true, disable file access time updates.
	MountDisableAccessTime *bool `yaml:"disableAccessTime,omitempty"`
	//   description: |
	//     Enable secure mount options (nosuid, nodev, noexec).
	//
	//     Defaults to true for better security.
	MountSecure *bool `yaml:"secure,omitempty"`
	//   description: |
	//     The user ID owning the mount point of the volume (`/var/mnt/<name>`).
	//
	//     Only the mount point itself is changed, not the files and directories under it.
	//     Containers running as different users can share the volume through a common `group`.
	//
	//     Defaults to `0` (root).
	//   examples:
	//     - value: 1000
	//   schema:
	//     type: integer
	//     minimum: 0
	//     maximum: 4294967294
	MountOwner *uint32 `yaml:"owner,omitempty"`
	//   description: |
	//     The group ID owning the mount point of the volume (`/var/mnt/<name>`).
	//
	//     Only the mount point itself is changed, not the files and directories under it.
	//
	//     Defaults to `0` (root).
	//   examples:
	//     - value: 2000
	//   schema:
	//     type: integer
	//     minimum: 0
	//     maximum: 4294967294
	MountGroup *uint32 `yaml:"group,omitempty"`
	//   description: |
	//     The permissions of the mount point of the volume (`/var/mnt/<name>`) in octal.
	//
	//     The value must be written in octal notation (`0o2770` or `02770`): an unprefixed
	//     number such as `2770` is rejected, as YAML would read it as decimal.
	//
	//     The setuid (`0o4000`), setgid (`0o2000`) and sticky (`0o1000`) bits may be set:
	//     with setgid, files created in the volume inherit the group of the mount point.
	//
	//     Defaults to `0o755`.
	//   schema:
	//     type: integer
	//     minimum: 0
	//     maximum: 4095 # 0o7777
	MountMode *UserVolumeMountMode `yaml:"mode,omitempty"`
}

// userVolumeDefaultMountMode is the mode of the mount point of a user volume which does not set one.
const userVolumeDefaultMountMode = 0o755

// userVolumeMaxID is the highest user or group ID of a user volume mount point.
//
// 4294967295 ((uid_t)-1) is not a valid ID: chown reads it as "leave unchanged".
const userVolumeMaxID uint32 = 4294967294

// userVolumeMountModeMask is the mask of the bits allowed in the mode of the mount point of a user volume.
const userVolumeMountModeMask = 0o7777

// UserVolumeMountMode holds the Unix mode bits of a user volume mount point (e.g. 0o2770), not an fs.FileMode.
type UserVolumeMountMode uint32

// String converts the mode to an octal string.
func (mode UserVolumeMountMode) String() string {
	return "0o" + strconv.FormatUint(uint64(mode), 8)
}

// MarshalYAML encodes as an octal value.
func (mode UserVolumeMountMode) MarshalYAML() (any, error) {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!int",
		Value: mode.String(),
	}, nil
}

// UnmarshalYAML decodes the mode, which must be written in octal notation.
//
// YAML reads an unprefixed number such as 2770 as decimal, which would silently produce
// unrelated permission bits, so only octal literals (0o2770 or 02770) are accepted.
func (mode *UserVolumeMountMode) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode || !isOctalLiteral(value.Value) {
		return fmt.Errorf("mount.mode must be an octal number such as 0o2770, got %q", value.Value)
	}

	var v uint32

	if err := value.Decode(&v); err != nil {
		return fmt.Errorf("mount.mode must be an octal number such as 0o2770, got %q: %w", value.Value, err)
	}

	*mode = UserVolumeMountMode(v)

	return nil
}

// isOctalLiteral reports whether s is written as an octal number: 0, 0o-prefixed or 0-prefixed (YAML 1.1).
func isOctalLiteral(s string) bool {
	if s == "0" || strings.HasPrefix(s, "0o") {
		return true
	}

	return len(s) > 1 && s[0] == '0' && s[1] >= '0' && s[1] <= '7'
}

// FileMode converts the Unix mode bits to fs.FileMode.
func (mode UserVolumeMountMode) FileMode() fs.FileMode {
	fileMode := fs.FileMode(mode) & fs.ModePerm

	if mode&0o4000 != 0 {
		fileMode |= fs.ModeSetuid
	}

	if mode&0o2000 != 0 {
		fileMode |= fs.ModeSetgid
	}

	if mode&0o1000 != 0 {
		fileMode |= fs.ModeSticky
	}

	return fileMode
}

// NewUserVolumeConfigV1Alpha1 creates a new user volume config document.
func NewUserVolumeConfigV1Alpha1() *UserVolumeConfigV1Alpha1 {
	return &UserVolumeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       UserVolumeConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

const userVolumeName = "local-data"

func exampleUserVolumeConfigV1Alpha1Partition() *UserVolumeConfigV1Alpha1 {
	cfg := NewUserVolumeConfigV1Alpha1()
	cfg.MetaName = userVolumeName
	cfg.VolumeType = new(block.VolumeTypePartition)
	cfg.ProvisioningSpec = ProvisioningSpec{
		DiskSelectorSpec: DiskSelector{
			Match: cel.MustExpression(cel.ParseBooleanExpression(`disk.transport == "nvme"`, celenv.DiskLocator())),
		},
		ProvisioningMaxSize: MustSize("50GiB"),
	}
	cfg.FilesystemSpec = FilesystemSpec{
		FilesystemType: block.FilesystemTypeXFS,
	}
	cfg.EncryptionSpec = EncryptionSpec{
		EncryptionProvider: block.EncryptionProviderLUKS2,
		EncryptionKeys: []EncryptionKey{
			{
				KeySlot: 0,
				KeyTPM:  &EncryptionKeyTPM{},
			},
			{
				KeySlot: 1,
				KeyStatic: &EncryptionKeyStatic{
					KeyData: "topsecret",
				},
			},
		},
	}

	return cfg
}

func exampleUserVolumeConfigV1Alpha1Directory() *UserVolumeConfigV1Alpha1 {
	cfg := NewUserVolumeConfigV1Alpha1()
	cfg.MetaName = userVolumeName
	cfg.VolumeType = new(block.VolumeTypeDirectory)

	return cfg
}

func exampleUserVolumeConfigV1Alpha1Owned() *UserVolumeConfigV1Alpha1 {
	cfg := NewUserVolumeConfigV1Alpha1()
	cfg.MetaName = userVolumeName
	cfg.VolumeType = new(block.VolumeTypeDirectory)
	cfg.MountSpec = UserMountSpec{
		MountOwner: new(uint32(1000)),
		MountGroup: new(uint32(2000)),
		MountMode:  new(UserVolumeMountMode(0o2770)),
	}

	return cfg
}

func exampleUserVolumeConfigV1Alpha1Tmpfs() *UserVolumeConfigV1Alpha1 {
	cfg := NewUserVolumeConfigV1Alpha1()
	cfg.MetaName = userVolumeName
	cfg.VolumeType = new(block.VolumeTypeTmpfs)
	cfg.ProvisioningSpec = ProvisioningSpec{
		ProvisioningMaxSize: MustSize("64MiB"),
	}

	return cfg
}

func exampleUserVolumeConfigV1Alpha1Disk() *UserVolumeConfigV1Alpha1 {
	cfg := NewUserVolumeConfigV1Alpha1()
	cfg.MetaName = userVolumeName
	cfg.VolumeType = new(block.VolumeTypeDisk)
	cfg.ProvisioningSpec = ProvisioningSpec{
		DiskSelectorSpec: DiskSelector{
			Match: cel.MustExpression(cel.ParseBooleanExpression(`disk.transport == "nvme"`, celenv.DiskLocator())),
		},
	}
	cfg.FilesystemSpec = FilesystemSpec{
		FilesystemType: block.FilesystemTypeXFS,
	}
	cfg.EncryptionSpec = EncryptionSpec{
		EncryptionProvider: block.EncryptionProviderLUKS2,
		EncryptionKeys: []EncryptionKey{
			{
				KeySlot: 0,
				KeyTPM:  &EncryptionKeyTPM{},
			},
			{
				KeySlot: 1,
				KeyStatic: &EncryptionKeyStatic{
					KeyData: "topsecret",
				},
			},
		},
	}

	return cfg
}

// Name implements config.NamedDocument interface.
func (s *UserVolumeConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *UserVolumeConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Redact implements config.SecretDocument interface.
func (s *UserVolumeConfigV1Alpha1) Redact(replacement string) {
	s.EncryptionSpec.Redact(replacement)
}

// ConflictsWithKinds implements config.ConflictingDocument interface.
func (s *UserVolumeConfigV1Alpha1) ConflictsWithKinds() []string {
	return conflictingVolumeKinds(UserVolumeConfigKind)
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo,cyclop
func (s *UserVolumeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		warnings         []string
		validationErrors error
	)

	validationErrors = errors.Join(validationErrors, validateVolumeName(s.MetaName, constants.UserVolumePrefix))

	vtype := block.VolumeTypePartition
	if s.VolumeType != nil {
		vtype = *s.VolumeType
	}

	switch vtype {
	case block.VolumeTypeDirectory:
		if !s.ProvisioningSpec.IsZero() {
			validationErrors = errors.Join(validationErrors, errors.New("provisioning spec is invalid for volumeType directory"))
		}

		if !s.EncryptionSpec.IsZero() {
			validationErrors = errors.Join(validationErrors, errors.New("encryption spec is invalid for volumeType directory"))
		}

		if !s.FilesystemSpec.IsZero() {
			validationErrors = errors.Join(validationErrors, errors.New("filesystem spec is invalid for volumeType directory"))
		}

		if s.MountSpec.MountDisableAccessTime != nil {
			validationErrors = errors.Join(validationErrors, errors.New("mount.disableAccessTime is invalid for volumeType directory"))
		}

	case block.VolumeTypeDisk:
		extraWarnings, extraErrors := s.ProvisioningSpec.Validate(true, false)

		warnings = append(warnings, extraWarnings...)
		validationErrors = errors.Join(validationErrors, extraErrors)

		extraWarnings, extraErrors = s.FilesystemSpec.Validate()
		warnings = append(warnings, extraWarnings...)
		validationErrors = errors.Join(validationErrors, extraErrors)

		extraWarnings, extraErrors = s.EncryptionSpec.Validate()
		warnings = append(warnings, extraWarnings...)
		validationErrors = errors.Join(validationErrors, extraErrors)

	case block.VolumeTypePartition:
		extraWarnings, extraErrors := s.ProvisioningSpec.Validate(true, true)

		warnings = append(warnings, extraWarnings...)
		validationErrors = errors.Join(validationErrors, extraErrors)

		extraWarnings, extraErrors = s.FilesystemSpec.Validate()
		warnings = append(warnings, extraWarnings...)
		validationErrors = errors.Join(validationErrors, extraErrors)

		extraWarnings, extraErrors = s.EncryptionSpec.Validate()
		warnings = append(warnings, extraWarnings...)
		validationErrors = errors.Join(validationErrors, extraErrors)

	case block.VolumeTypeTmpfs:
		extraWarnings, extraErrors := s.validateTmpfs()

		warnings = append(warnings, extraWarnings...)
		validationErrors = errors.Join(validationErrors, extraErrors)

	case block.VolumeTypeSymlink, block.VolumeTypeOverlay, block.VolumeTypeExternal:
		fallthrough

	default:
		validationErrors = errors.Join(validationErrors, fmt.Errorf("unsupported volume type %q", vtype))
	}

	if err := s.MountSpec.Validate(); err != nil {
		validationErrors = errors.Join(validationErrors, err)
	}

	if err := s.TrimSpec.Validate(); err != nil {
		validationErrors = errors.Join(validationErrors, err)
	}

	if err := s.ScrubSpec.Validate(); err != nil {
		validationErrors = errors.Join(validationErrors, err)
	}

	return warnings, validationErrors
}

// validateTmpfs validates a tmpfs user volume.
//
// A tmpfs is backed by memory: the only provisioning setting is the size (provisioning.maxSize), which is required,
// as an unbounded tmpfs may take up to half of the memory.
//
// The kernel doesn't cap the size against the memory, and tmpfs memory is only reclaimed by swapping it out
// (the OOM killer can't free it, as the files outlive the processes),
// so a percentage above tmpfsWarnPercent (the kernel default size) is reported as a warning.
// A size in bytes can't be checked, as the size of the memory is unknown here.
func (s *UserVolumeConfigV1Alpha1) validateTmpfs() ([]string, error) {
	warnings, validationErrors := s.validateTmpfsSize()

	for _, invalid := range []struct {
		isSet bool
		name  string
	}{
		{!s.ProvisioningSpec.ProvisioningMinSize.IsZero(), "provisioning.minSize"},
		{s.ProvisioningSpec.ProvisioningGrow != nil, "provisioning.grow"},
		{!s.ProvisioningSpec.DiskSelectorSpec.Match.IsZero(), "provisioning.diskSelector"},
		{!s.EncryptionSpec.IsZero(), "encryption spec"},
		{!s.FilesystemSpec.IsZero(), "filesystem spec"},
		{s.TrimSpec != nil, "trim spec"},
		{s.ScrubSpec != nil, "scrub spec"},
	} {
		if invalid.isSet {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("%s is invalid for volumeType tmpfs", invalid.name))
		}
	}

	return warnings, validationErrors
}

// validateTmpfsSize validates provisioning.maxSize of a tmpfs user volume.
func (s *UserVolumeConfigV1Alpha1) validateTmpfsSize() ([]string, error) {
	maxSize := s.ProvisioningSpec.ProvisioningMaxSize

	switch {
	case maxSize.IsZero():
		return nil, errors.New("provisioning.maxSize is required for volumeType tmpfs")
	case maxSize.IsNegative():
		return nil, errors.New("provisioning.maxSize cannot be negative for volumeType tmpfs")
	}

	if percent, ok := maxSize.RelativeValue(); ok && percent > tmpfsWarnPercent {
		return []string{fmt.Sprintf(
			"provisioning.maxSize %d%% for volumeType tmpfs is above %d%% of the memory: tmpfs memory is not freed by the OOM killer and can only be swapped out, so it might exhaust the memory",
			percent, tmpfsWarnPercent,
		)}, nil
	}

	return nil, nil
}

// UserVolumeConfigSignal is a signal for user volume config.
func (s *UserVolumeConfigV1Alpha1) UserVolumeConfigSignal() {}

// Type implements config.UserVolumeConfig interface.
func (s *UserVolumeConfigV1Alpha1) Type() optional.Optional[VolumeType] {
	if s.VolumeType == nil {
		return optional.None[VolumeType]()
	}

	return optional.Some(*s.VolumeType)
}

// Provisioning implements config.UserVolumeConfig interface.
func (s *UserVolumeConfigV1Alpha1) Provisioning() config.VolumeProvisioningConfig {
	return s.ProvisioningSpec
}

// Filesystem implements config.UserVolumeConfig interface.
func (s *UserVolumeConfigV1Alpha1) Filesystem() config.FilesystemConfig {
	return s.FilesystemSpec
}

// Encryption implements config.UserVolumeConfig interface.
func (s *UserVolumeConfigV1Alpha1) Encryption() config.EncryptionConfig {
	if s.EncryptionSpec.EncryptionProvider == block.EncryptionProviderNone {
		return nil
	}

	return s.EncryptionSpec
}

// Mount implements config.UserVolumeConfig interface.
func (s *UserVolumeConfigV1Alpha1) Mount() config.UserVolumeMountConfig {
	return s.MountSpec
}

// Trim implements config.UserVolumeConfig interface.
func (s *UserVolumeConfigV1Alpha1) Trim() config.VolumeTrimConfig {
	if s.TrimSpec == nil {
		return nil
	}

	return s.TrimSpec
}

// Scrub implements config.UserVolumeConfig interface.
func (s *UserVolumeConfigV1Alpha1) Scrub() config.VolumeScrubConfig {
	if s.ScrubSpec == nil {
		return nil
	}

	return s.ScrubSpec
}

// FilesystemSpec configures the filesystem for the volume.
type FilesystemSpec struct {
	//   description: |
	//     Filesystem type. Default is `xfs`.
	//   values:
	//     - ext4
	//     - xfs
	//     - btrfs
	FilesystemType block.FilesystemType `yaml:"type,omitempty"`
	//   description: |
	//     Enables project quota support, valid only for 'xfs' filesystem.
	//
	//     Note: changing this value might require a full remount of the filesystem.
	ProjectQuotaSupportConfig *bool `yaml:"projectQuotaSupport,omitempty"`
	//   description: |
	//     XFS-specific filesystem options, valid only for 'xfs' filesystem.
	XFSSpec *XFSSpec `yaml:"xfs,omitempty"`
}

// XFSSpec configures XFS-specific filesystem options.
type XFSSpec struct {
	//  description: |
	//    The minimum size of an XFS allocation group.
	//
	//    On non-rotational devices `mkfs.xfs` sizes the allocation group count to the number of
	//    CPUs, which on machines with many cores and a modest disk yields hundreds of tiny
	//    allocation groups. Talos bounds the allocation group size from below to keep the geometry
	//    sane; this option overrides that bound.
	//
	//    Set to zero to use the `mkfs.xfs` defaults unchanged.
	//
	//    Note: this only affects volumes at the time they are formatted.
	//
	//    Size is specified in bytes, but can be expressed in human readable format, e.g. 100MB.
	//  examples:
	//    - value: >
	//        "128GiB"
	//  schema:
	//    type: string
	MinAllocationGroupSizeConfig meta.ByteSize `yaml:"minAllocationGroupSize,omitempty"`
}

// IsZero checks if the filesystem spec is zero.
func (s FilesystemSpec) IsZero() bool {
	return s.FilesystemType == block.FilesystemTypeNone && s.ProjectQuotaSupportConfig == nil && s.XFSSpec == nil
}

// Type implements config.FilesystemConfig interface.
func (s FilesystemSpec) Type() block.FilesystemType {
	if s.FilesystemType == block.FilesystemTypeNone {
		return block.FilesystemTypeXFS
	}

	return s.FilesystemType
}

// ProjectQuotaSupport implements config.FilesysteemConfig interface.
func (s FilesystemSpec) ProjectQuotaSupport() bool {
	return pointer.SafeDeref(s.ProjectQuotaSupportConfig)
}

// XFS implements config.FilesystemConfig interface.
func (s FilesystemSpec) XFS() config.XFSFilesystemConfig {
	if s.XFSSpec == nil {
		return nil
	}

	return s.XFSSpec
}

// Validate implements config.Validator interface.
func (s FilesystemSpec) Validate() ([]string, error) {
	switch s.FilesystemType { //nolint:exhaustive
	case block.FilesystemTypeNone:
	case block.FilesystemTypeXFS:
	case block.FilesystemTypeEXT4:
	case block.FilesystemTypeBtrfs:
	default:
		return nil, fmt.Errorf("unsupported filesystem type: %s", s.FilesystemType)
	}

	if pointer.SafeDeref(s.ProjectQuotaSupportConfig) && s.Type() != block.FilesystemTypeXFS {
		return nil, fmt.Errorf("project quota support is only available for xfs filesystem")
	}

	if s.XFSSpec != nil && s.Type() != block.FilesystemTypeXFS {
		return nil, fmt.Errorf("xfs options are only available for xfs filesystem")
	}

	return nil, nil
}

// MinAllocationGroupSize implements config.XFSFilesystemConfig interface.
func (s *XFSSpec) MinAllocationGroupSize() optional.Optional[uint64] {
	if s.MinAllocationGroupSizeConfig.IsZero() {
		return optional.None[uint64]()
	}

	return optional.Some(s.MinAllocationGroupSizeConfig.Value())
}

// IsZero checks if the mount spec is zero.
func (s UserMountSpec) IsZero() bool {
	return s.MountDisableAccessTime == nil && s.MountSecure == nil &&
		s.MountOwner == nil && s.MountGroup == nil && s.MountMode == nil
}

// Owner implements config.UserVolumeMountConfig interface.
func (s UserMountSpec) Owner() uint32 {
	return pointer.SafeDeref(s.MountOwner)
}

// Group implements config.UserVolumeMountConfig interface.
func (s UserMountSpec) Group() uint32 {
	return pointer.SafeDeref(s.MountGroup)
}

// Mode implements config.UserVolumeMountConfig interface.
func (s UserMountSpec) Mode() fs.FileMode {
	if s.MountMode == nil {
		return UserVolumeMountMode(userVolumeDefaultMountMode).FileMode()
	}

	return s.MountMode.FileMode()
}

// Validate validates the mount spec.
func (s UserMountSpec) Validate() error {
	var validationErrors error

	if s.MountOwner != nil && *s.MountOwner > userVolumeMaxID {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("mount.owner must be at most %d, got %d", userVolumeMaxID, *s.MountOwner))
	}

	if s.MountGroup != nil && *s.MountGroup > userVolumeMaxID {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("mount.group must be at most %d, got %d", userVolumeMaxID, *s.MountGroup))
	}

	if s.MountMode != nil && *s.MountMode&^userVolumeMountModeMask != 0 {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("mount.mode %s may only contain permission, setuid, setgid and sticky bits", *s.MountMode))
	}

	return validationErrors
}

// DisableAccessTime implements config.UserVolumeMountConfig interface.
func (s UserMountSpec) DisableAccessTime() bool {
	return pointer.SafeDeref(s.MountDisableAccessTime)
}

// Secure implements config.UserVolumeMountConfig interface.
func (s UserMountSpec) Secure() bool {
	if s.MountSecure == nil {
		return true
	}

	return *s.MountSecure
}

// validateVolumeName validates a volume name, given the volume ID prefix.
func validateVolumeName(name, idPrefix string) error {
	var validationErrors error

	maxNameLength := constants.PartitionLabelLength - len(idPrefix)

	if name == "" {
		validationErrors = errors.Join(validationErrors, errors.New("name is required"))
	}

	if len(name) < 1 || len(name) > maxNameLength {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("name must be between 1 and %d characters long", maxNameLength))
	}

	if strings.ContainsFunc(name, isInvalidVolumeNameRune) {
		validationErrors = errors.Join(validationErrors, errors.New("name can only contain lowercase and uppercase ASCII letters, digits, and hyphens"))
	}

	return validationErrors
}

func isInvalidVolumeNameRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return false
	case r >= 'A' && r <= 'Z':
		return false
	case r >= '0' && r <= '9':
		return false
	case r == '-':
		return false
	default: // invalid symbol
		return true
	}
}
