// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// VolumeConfigKind is a config document kind.
const VolumeConfigKind = "VolumeConfig"

func init() {
	registry.Register(VolumeConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &VolumeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.VolumeConfig                 = &VolumeConfigV1Alpha1{}
	_ config.NamedDocument                = &VolumeConfigV1Alpha1{}
	_ config.Validator                    = &VolumeConfigV1Alpha1{}
	_ config.RuntimeValidator             = &VolumeConfigV1Alpha1{}
	_ config.SecretDocument               = &VolumeConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &VolumeConfigV1Alpha1{}
)

// VolumeConfigV1Alpha1 is a system volume configuration document.
//
//	description: |
//	  Note: at the moment, only `STATE`, `EPHEMERAL`, `IMAGECACHE`, `ETCD`, `CRI` and `KUBELET`
//	  system volumes are supported. The `ETCD`, `CRI` and `KUBELET` volumes default to a directory
//	  under `EPHEMERAL`, and can be placed on a dedicated partition by specifying `provisioning`.
//	  The backing of these volumes (directory vs. dedicated partition) can only be chosen at cluster
//	  creation time: changing it on an already-provisioned node is not supported.
//	examples:
//	  - value: exampleVolumeConfigEphemeralV1Alpha1()
//	alias: VolumeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/VolumeConfig
type VolumeConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the volume.
	MetaName string `yaml:"name"`
	//   description: |
	//     The provisioning describes how the volume is provisioned.
	ProvisioningSpec ProvisioningSpec `yaml:"provisioning,omitempty"`
	//   description: |
	//     The encryption describes how the volume is encrypted.
	EncryptionSpec EncryptionSpec `yaml:"encryption,omitempty"`
	//   description: |
	//     The mount describes additional mount options.
	MountSpec MountSpec `yaml:"mount,omitempty"`
	//   description: |
	//     The trim describes the per-volume filesystem trim (fstrim) configuration.
	TrimSpec *TrimConfig `yaml:"trim,omitempty"`
}

// MountSpec describes how the volume is mounted.
type MountSpec struct {
	//   description: |
	//     Enable secure mount options (nosuid, nodev).
	//
	//     Defaults to true for better security.
	//     Supported only for EPHEMERAL volume.
	MountSecure *bool `yaml:"secure,omitempty"`
	//   description: |
	//     If true, disable file access time updates.
	//
	//     Supported only for EPHEMERAL volume.
	MountDisableAccessTime *bool `yaml:"disableAccessTime,omitempty"`
}

// ProvisioningSpec describes how the volume is provisioned.
type ProvisioningSpec struct {
	//   description: |
	//     The disk selector expression.
	DiskSelectorSpec DiskSelector `yaml:"diskSelector,omitempty"`
	//   description: |
	//    Should the volume grow to the size of the disk (if possible).
	ProvisioningGrow *bool `yaml:"grow,omitempty"`
	//  description: |
	//    The minimum size of the volume.
	//
	//    Size is specified in bytes, but can be expressed in human readable format, e.g. 100MB.
	//  examples:
	//    - value: >
	//        "2.5GiB"
	//  schema:
	//    type: string
	ProvisioningMinSize ByteSize `yaml:"minSize,omitempty"`
	//  description: |
	//    The maximum size of the volume, if not specified the volume can grow to the size of the
	//    disk.
	//
	//    Size is specified in bytes or in percents. It can be expressed in human readable format, e.g. 100MB.
	//  examples:
	//    - value: >
	//        "50GiB"
	//    - value: >
	//        "80%"
	//  schema:
	//    type: string
	ProvisioningMaxSize Size `yaml:"maxSize,omitempty"`
}

// MaxSizeNegative returns true if the maximum size is negative.
func (p ProvisioningSpec) MaxSizeNegative() bool {
	return p.ProvisioningMaxSize.IsNegative()
}

// DiskSelector selects a disk for the volume.
type DiskSelector struct {
	//   description: |
	//     The Common Expression Language (CEL) expression to match the disk.
	//   schema:
	//     type: string
	//   examples:
	//    - value: >
	//        exampleDiskSelector1()
	//      name: match disks with size between 120GB and 1TB
	//    - value: >
	//        exampleDiskSelector2()
	//      name: match SATA disks that are not rotational and not system disks
	Match cel.Expression `yaml:"match,omitempty"`
}

// NewVolumeConfigV1Alpha1 creates a new volume config document.
func NewVolumeConfigV1Alpha1() *VolumeConfigV1Alpha1 {
	return &VolumeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       VolumeConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleVolumeConfigEphemeralV1Alpha1() *VolumeConfigV1Alpha1 {
	cfg := NewVolumeConfigV1Alpha1()
	cfg.MetaName = constants.EphemeralPartitionLabel
	cfg.ProvisioningSpec = ProvisioningSpec{
		DiskSelectorSpec: DiskSelector{
			Match: cel.MustExpression(cel.ParseBooleanExpression(`disk.transport == "nvme"`, celenv.DiskLocator())),
		},
		ProvisioningMaxSize: MustSize("50GiB"),
	}

	return cfg
}

func exampleDiskSelector1() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`disk.size > 120u * GB && disk.size < 1u * TB`, celenv.DiskLocator()))
}

func exampleDiskSelector2() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`disk.transport == "sata" && !disk.rotational && !system_disk`, celenv.DiskLocator()))
}

// Name implements config.NamedDocument interface.
func (s *VolumeConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *VolumeConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Redact implements config.SecretDocument interface.
func (s *VolumeConfigV1Alpha1) Redact(replacement string) {
	s.EncryptionSpec.Redact(replacement)
}

// Validate implements config.Validator interface.
func (s *VolumeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	allowedVolumes := []string{
		constants.StatePartitionLabel,
		constants.EphemeralPartitionLabel,
		constants.ImageCachePartitionLabel,
		constants.EtcdDataVolumeID,
		constants.CRIContainerdVolumeID,
		constants.KubeletDataVolumeID,
	}

	if slices.Index(allowedVolumes, s.MetaName) == -1 {
		return nil, fmt.Errorf("only %q volumes are supported", allowedVolumes)
	}

	var (
		warnings         []string //nolint:prealloc
		validationErrors error
	)

	validationErrors = errors.Join(validationErrors, s.validateVolumeConstraints())

	extraWarnings, extraErrors := s.ProvisioningSpec.Validate(false, true)
	warnings = append(warnings, extraWarnings...)
	validationErrors = errors.Join(validationErrors, extraErrors)

	extraWarnings, extraErrors = s.EncryptionSpec.Validate()
	warnings = append(warnings, extraWarnings...)
	validationErrors = errors.Join(validationErrors, extraErrors)

	if err := s.TrimSpec.Validate(); err != nil {
		validationErrors = errors.Join(validationErrors, err)
	}

	return warnings, validationErrors
}

// mountNotAllowed returns an error if mount config is set for a volume that does not support it.
func mountNotAllowed(name string, mount MountSpec) error {
	if mount != (MountSpec{}) {
		return fmt.Errorf("mount config is not allowed for the %q volume", name)
	}

	return nil
}

// validateVolumeConstraints validates the per-volume constraints on which config sections are allowed.
func (s *VolumeConfigV1Alpha1) validateVolumeConstraints() error {
	var validationErrors error

	switch s.MetaName {
	case constants.StatePartitionLabel:
		// no provisioning config is allowed for the state partition.
		if !s.ProvisioningSpec.IsZero() {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("provisioning config is not allowed for the %q volume", s.MetaName))
		}

		for _, key := range s.EncryptionSpec.EncryptionKeys {
			if pointer.SafeDeref(key.KeyLockToSTATE) {
				// state-locked keys are not allowed
				validationErrors = errors.Join(validationErrors, fmt.Errorf("state-locked key is not allowed for the %q volume", s.MetaName))
			}
		}

		validationErrors = errors.Join(validationErrors, mountNotAllowed(s.MetaName, s.MountSpec))
	case constants.ImageCachePartitionLabel:
		validationErrors = errors.Join(validationErrors, mountNotAllowed(s.MetaName, s.MountSpec))
	case constants.EtcdDataVolumeID, constants.CRIContainerdVolumeID, constants.KubeletDataVolumeID:
		// these volumes default to a directory under EPHEMERAL and can be placed on a dedicated
		// partition via provisioning (optionally encrypted); mount config is not supported.
		validationErrors = errors.Join(validationErrors, mountNotAllowed(s.MetaName, s.MountSpec))
	}

	return validationErrors
}

// promotableSystemVolumes are the system volumes that default to a directory under the EPHEMERAL
// volume but may instead be placed on a dedicated partition (via provisioning) at cluster creation.
var promotableSystemVolumes = []string{
	constants.EtcdDataVolumeID,
	constants.CRIContainerdVolumeID,
	constants.KubeletDataVolumeID,
}

// ProvisioningRequested reports whether provisioning is explicitly configured for a volume.
//
// For the promotable system volumes (ETCD, CRI, KUBELET) this is the opt-in signal to place the
// volume on a dedicated partition instead of a directory under EPHEMERAL. The volume controller and
// config validation share this predicate so they agree on what counts as a dedicated partition.
func ProvisioningRequested(p config.VolumeProvisioningConfig) bool {
	return p.DiskSelector().IsPresent() ||
		p.Grow().IsPresent() ||
		p.MinSize().IsPresent() ||
		p.MaxSize().IsPresent()
}

// RuntimeValidate implements config.RuntimeValidator interface.
//
// For the promotable system volumes (ETCD, CRI, KUBELET) it enforces "create-only" semantics: the
// backing of the volume — a directory under EPHEMERAL vs. a dedicated partition — is fixed at cluster
// creation and cannot be changed on an already-provisioned node. Migrating an existing system volume
// to or from a dedicated partition would orphan its on-disk state (and, for etcd, risk quorum), so it
// is rejected here. Live migration may be supported later by a dedicated controller.
//
// The check compares the desired backing (partition if provisioning is requested, directory
// otherwise) against the actual VolumeStatus. If the volume is not established yet (cluster creation
// or boot, before the volume manager has processed it) there is nothing to conflict with, so it is
// allowed.
func (s *VolumeConfigV1Alpha1) RuntimeValidate(ctx context.Context, st state.State, _ validation.RuntimeMode, _ ...validation.Option) ([]string, error) {
	if !slices.Contains(promotableSystemVolumes, s.MetaName) {
		return nil, nil
	}

	desiredType := block.VolumeTypeDirectory
	if ProvisioningRequested(s.ProvisioningSpec) {
		desiredType = block.VolumeTypePartition
	}

	volumeStatus, err := safe.StateGetByID[*block.VolumeStatus](ctx, st, s.MetaName)
	if err != nil {
		if state.IsNotFoundError(err) {
			// the volume is not established yet (cluster creation / boot): nothing to conflict with.
			return nil, nil
		}

		return nil, err
	}

	// only compare against a settled volume; an in-flight volume is not yet established.
	if volumeStatus.TypedSpec().Phase != block.VolumePhaseReady {
		return nil, nil
	}

	if currentType := volumeStatus.TypedSpec().Type; currentType != desiredType {
		return nil, fmt.Errorf(
			"the backing of the %q system volume cannot be changed after creation (current: %s, requested: %s); "+
				"migrating an existing system volume to or from a dedicated partition is not supported",
			s.MetaName, currentType, desiredType,
		)
	}

	return nil, nil
}

// Provisioning implements config.VolumeConfig interface.
func (s *VolumeConfigV1Alpha1) Provisioning() config.VolumeProvisioningConfig {
	return s.ProvisioningSpec
}

// Encryption implements config.VolumeConfig interface.
func (s *VolumeConfigV1Alpha1) Encryption() config.EncryptionConfig {
	if s.EncryptionSpec.EncryptionProvider == block.EncryptionProviderNone {
		return nil
	}

	return s.EncryptionSpec
}

// Mount implements config.VolumeConfig interface.
func (s *VolumeConfigV1Alpha1) Mount() config.VolumeMountConfig {
	return s.MountSpec
}

// Trim implements config.VolumeConfig interface.
func (s *VolumeConfigV1Alpha1) Trim() config.VolumeTrimConfig {
	if s.TrimSpec == nil {
		return nil
	}

	return s.TrimSpec
}

// Validate the provisioning spec.
//
//nolint:gocyclo
func (p ProvisioningSpec) Validate(required bool, sizeSupported bool) ([]string, error) {
	var validationErrors error

	if !p.DiskSelectorSpec.Match.IsZero() {
		if err := p.DiskSelectorSpec.Match.ParseBool(celenv.DiskLocator()); err != nil {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("disk selector is invalid: %w", err))
		}
	} else if required {
		validationErrors = errors.Join(validationErrors, errors.New("disk selector is required"))
	}

	if sizeSupported {
		if !p.ProvisioningMinSize.IsZero() && !p.ProvisioningMaxSize.IsZero() && !p.ProvisioningMaxSize.IsRelative() && !p.ProvisioningMaxSize.IsNegative() {
			if p.ProvisioningMinSize.Value() > p.ProvisioningMaxSize.Value() {
				validationErrors = errors.Join(validationErrors, errors.New("min size is greater than max size"))
			}
		} else if required && p.ProvisioningMinSize.IsZero() && p.ProvisioningMaxSize.IsZero() {
			validationErrors = errors.Join(validationErrors, errors.New("min size or max size is required"))
		}

		if p.ProvisioningMinSize.IsNegative() {
			validationErrors = errors.Join(validationErrors, errors.New("min size cannot be negative"))
		}
	} else if !p.ProvisioningMinSize.IsZero() || !p.ProvisioningMaxSize.IsZero() || p.Grow().IsPresent() {
		validationErrors = errors.Join(validationErrors, errors.New("min size, max size and grow are not supported"))
	}

	return nil, validationErrors
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *VolumeConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Config *v1alpha1.Config) error {
	if !slices.Contains([]string{constants.StatePartitionLabel, constants.EphemeralPartitionLabel}, s.MetaName) {
		// only STATE and EPHEMERAL volumes can conflict with legacy config.
		return nil
	}

	if s.Encryption() == nil {
		// no encryption configured, no conflict.
		return nil
	}

	legacy := v1alpha1Config.Machine().SystemDiskEncryption().Get(s.MetaName)
	if legacy != nil {
		return fmt.Errorf("system disk encryption for %q is configured in both v1alpha1.Config and VolumeConfig", s.MetaName)
	}

	return nil
}

// IsZero checks if the provisioning spec is zero.
func (p ProvisioningSpec) IsZero() bool {
	return p.ProvisioningGrow == nil && p.ProvisioningMaxSize.IsZero() && p.ProvisioningMinSize.IsZero() && p.DiskSelectorSpec.Match.IsZero()
}

// DiskSelector implements config.VolumeProvisioningConfig interface.
func (p ProvisioningSpec) DiskSelector() optional.Optional[cel.Expression] {
	if p.DiskSelectorSpec.Match.IsZero() {
		return optional.None[cel.Expression]()
	}

	return optional.Some(p.DiskSelectorSpec.Match)
}

// Grow implements config.VolumeProvisioningConfig interface.
func (p ProvisioningSpec) Grow() optional.Optional[bool] {
	if p.ProvisioningGrow == nil {
		return optional.None[bool]()
	}

	return optional.Some(*p.ProvisioningGrow)
}

// MinSize implements config.VolumeProvisioningConfig interface.
func (p ProvisioningSpec) MinSize() optional.Optional[uint64] {
	if p.ProvisioningMinSize.IsZero() {
		return optional.None[uint64]()
	}

	return optional.Some(p.ProvisioningMinSize.Value())
}

// MaxSize implements config.VolumeProvisioningConfig interface.
func (p ProvisioningSpec) MaxSize() optional.Optional[uint64] {
	if p.ProvisioningMaxSize.IsZero() {
		return optional.None[uint64]()
	}

	return optional.Some(p.ProvisioningMaxSize.Value())
}

// RelativeMaxSize implements config.VolumeProvisioningConfig interface.
func (p ProvisioningSpec) RelativeMaxSize() optional.Optional[uint64] {
	if p.ProvisioningMaxSize.IsZero() {
		return optional.None[uint64]()
	}

	val, ok := p.ProvisioningMaxSize.RelativeValue()
	if !ok {
		return optional.None[uint64]()
	}

	return optional.Some(val)
}

// Secure implements config.VolumeMountConfig interface.
func (s MountSpec) Secure() bool {
	if s.MountSecure == nil {
		return true
	}

	return *s.MountSecure
}

// DisableAccessTime implements config.VolumeMountConfig interface.
func (s MountSpec) DisableAccessTime() bool {
	return pointer.SafeDeref(s.MountDisableAccessTime)
}
