// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// LVMVolumeConfigKind is a config document kind.
const LVMVolumeConfigKind = "LVMVolumeConfig"

func init() {
	registry.Register(LVMVolumeConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &LVMVolumeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NamedDocument = &LVMVolumeConfigV1Alpha1{}
	_ config.Validator     = &LVMVolumeConfigV1Alpha1{}
)

// LVType is an alias for block.LVType.
type LVType = block.LVType

// LVMPhysicalVolumeConfig describes an LVM physical volume configuration.
type LVMPhysicalVolumeConfig struct {
	//   description: |
	//     Optional name for the physical volume for referencing in volume groups.
	//     If not specified, the device path is used as the identifier.
	Name string `yaml:"name,omitempty"`
	//   description: |
	//     Device path for the physical volume.
	//   examples:
	//     - value: '"/dev/sdb"'
	Device string `yaml:"device"`
}

// IsZero checks if the physical volume config is zero.
func (s LVMPhysicalVolumeConfig) IsZero() bool {
	return s.Device == "" && s.Name == ""
}

// LVMVolumeGroupConfig describes an LVM volume group configuration.
type LVMVolumeGroupConfig struct {
	//   description: |
	//     Name of the volume group.
	Name string `yaml:"name"`
	//   description: |
	//     Physical volumes to include in this volume group.
	PhysicalVolumes []string `yaml:"physicalVolumes"`
	//   description: |
	//     Extent size for the volume group.
	//     If not specified, defaults to 4 MiB.
	//   examples:
	//     - value: '"4MiB"'
	//     - value: '"8MiB"'
	//   schema:
	//     type: string
	ExtentSize *ByteSize `yaml:"extentSize,omitempty"`
}

// IsZero checks if the volume group config is zero.
func (s LVMVolumeGroupConfig) IsZero() bool {
	return s.Name == "" && len(s.PhysicalVolumes) == 0
}

// LVMLogicalVolumeConfig describes an LVM logical volume configuration.
type LVMLogicalVolumeConfig struct {
	//   description: |
	//     Name of the logical volume.
	Name string `yaml:"name"`
	//   description: |
	//     Volume group name this logical volume belongs to.
	VolumeGroup string `yaml:"volumeGroup"`
	//   description: |
	//     Size of the logical volume.
	//   examples:
	//     - value: '"100GB"'
	//     - value: '"50GiB"'
	//   schema:
	//     type: string
	Size ByteSize `yaml:"size"`
	//   description: |
	//     Type of the logical volume.
	//     If not specified, defaults to linear.
	//   values:
	//     - linear
	//   schema:
	//     type: string
	Type *LVType `yaml:"type,omitempty"`
	//   description: |
	//     Filesystem configuration for the logical volume.
	Filesystem *FilesystemSpec `yaml:"filesystem,omitempty"`
	//   description: |
	//     Mount configuration for the logical volume.
	Mount *LVMMountSpec `yaml:"mount,omitempty"`
	//   description: |
	//     Encryption configuration for the logical volume.
	Encryption *EncryptionSpec `yaml:"encryption,omitempty"`
}

// IsZero checks if the logical volume config is zero.
func (s LVMLogicalVolumeConfig) IsZero() bool {
	return s.Name == "" && s.VolumeGroup == "" && s.Size.IsZero()
}

// LVMMountSpec describes mount configuration for an LVM logical volume.
type LVMMountSpec struct {
	//   description: |
	//     Path where the volume should be mounted.
	//   examples:
	//     - value: '"/var/lib/postgresql"'
	Path string `yaml:"path"`
	//   description: |
	//     Mount options.
	Options []string `yaml:"options,omitempty"`
	//   description: |
	//     Mount the volume as read-only.
	ReadOnly bool `yaml:"readOnly,omitempty"`
}

// IsZero checks if the mount spec is zero.
func (s LVMMountSpec) IsZero() bool {
	return s.Path == ""
}

// LVMVolumeConfigV1Alpha1 is an LVM volume configuration document.
//
//	description: |
//	  LVM volume configuration allows configuring LVM physical volumes, volume groups, and logical volumes.
//	  Each component is optional, allowing Talos to manage only what you specify.
//	  For example, you can manage only PVs and VGs, letting a CSI driver manage logical volumes.
//	examples:
//	  - value: exampleLVMVolumeConfigSimple()
//	  - value: exampleLVMVolumeConfigForCSI()
//	alias: LVMVolumeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/LVMVolumeConfig
type LVMVolumeConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the LVM configuration.
	//
	//     Name can only contain lowercase and uppercase ASCII letters, digits, and hyphens.
	MetaName string `yaml:"name"`
	//   description: |
	//     Physical volumes to create.
	PhysicalVolumes []LVMPhysicalVolumeConfig `yaml:"physicalVolumes,omitempty"`
	//   description: |
	//     Volume groups to create.
	VolumeGroups []LVMVolumeGroupConfig `yaml:"volumeGroups,omitempty"`
	//   description: |
	//     Logical volumes to create.
	LogicalVolumes []LVMLogicalVolumeConfig `yaml:"logicalVolumes,omitempty"`
}

// NewLVMVolumeConfigV1Alpha1 creates a new LVM volume config document.
func NewLVMVolumeConfigV1Alpha1() *LVMVolumeConfigV1Alpha1 {
	return &LVMVolumeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       LVMVolumeConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

// exampleLVMVolumeConfigSimple creates a simple LVM configuration example.
func exampleLVMVolumeConfigSimple() *LVMVolumeConfigV1Alpha1 {
	cfg := NewLVMVolumeConfigV1Alpha1()
	cfg.MetaName = "database-storage"
	cfg.PhysicalVolumes = []LVMPhysicalVolumeConfig{
		{
			Device: "/dev/sdb",
		},
	}
	cfg.VolumeGroups = []LVMVolumeGroupConfig{
		{
			Name:            "data-vg",
			PhysicalVolumes: []string{"/dev/sdb"},
		},
	}
	cfg.LogicalVolumes = []LVMLogicalVolumeConfig{
		{
			Name:        "data-lv",
			VolumeGroup: "data-vg",
			Size:        MustByteSize("100GB"),
			Filesystem: &FilesystemSpec{
				FilesystemType: block.FilesystemTypeXFS,
			},
			Mount: &LVMMountSpec{
				Path: "/var/lib/data",
			},
		},
	}

	return cfg
}

// exampleLVMVolumeConfigForCSI creates an LVM configuration for use with CSI drivers.
// This example shows configuring only PVs and VGs, letting the CSI driver manage logical volumes.
func exampleLVMVolumeConfigForCSI() *LVMVolumeConfigV1Alpha1 {
	cfg := NewLVMVolumeConfigV1Alpha1()
	cfg.MetaName = "csi-storage"
	cfg.PhysicalVolumes = []LVMPhysicalVolumeConfig{
		{
			Device: "/dev/sdc",
		},
		{
			Device: "/dev/sdd",
		},
	}
	cfg.VolumeGroups = []LVMVolumeGroupConfig{
		{
			Name:            "csi-vg",
			PhysicalVolumes: []string{"/dev/sdc", "/dev/sdd"},
		},
	}
	// No logical volumes - CSI driver will manage them

	return cfg
}

// Name implements config.NamedDocument interface.
func (s *LVMVolumeConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *LVMVolumeConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

var (
	nameRegex    = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)
	lvmNameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
)

// Validate implements config.Validator interface.
//
//nolint:gocyclo,cyclop
func (s *LVMVolumeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		warnings         []string
		validationErrors error
	)

	// Validate config name
	if s.MetaName == "" {
		validationErrors = errors.Join(validationErrors, errors.New("name is required"))
	}

	// Track used devices and names to detect duplicates
	pvDevices := make(map[string]bool)
	pvNames := make(map[string]string)

	for i, pv := range s.PhysicalVolumes {
		if pv.Device == "" {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("physical volume at index %d: device is required", i))

			continue
		}

		if !strings.HasPrefix(pv.Device, "/dev/") {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("physical volume %q: device must start with /dev/", pv.Device))
		}

		if pvDevices[pv.Device] {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("physical volume %q: duplicate device", pv.Device))
		}

		pvDevices[pv.Device] = true

		if pv.Name != "" {
			if _, exists := pvNames[pv.Name]; exists {
				validationErrors = errors.Join(validationErrors, fmt.Errorf("physical volume %q: duplicate name", pv.Name))
			}

			pvNames[pv.Name] = pv.Device
		}
	}

	// Validate volume groups
	vgNames := make(map[string]*LVMVolumeGroupConfig)

	for i, vg := range s.VolumeGroups {
		if vg.Name == "" {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("volume group at index %d: name is required", i))

			continue
		}

		if _, exists := vgNames[vg.Name]; exists {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("volume group %q: duplicate name", vg.Name))
		}

		vgNames[vg.Name] = &s.VolumeGroups[i]

		if len(vg.PhysicalVolumes) == 0 {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("volume group %q: at least one physical volume is required", vg.Name))
		}

		for _, pvRef := range vg.PhysicalVolumes {
			// Check if it's a device path or a name
			if _, isDevice := pvDevices[pvRef]; !isDevice {
				if _, isName := pvNames[pvRef]; !isName {
					validationErrors = errors.Join(validationErrors, fmt.Errorf("volume group %q: physical volume %q not found", vg.Name, pvRef))
				}
			}
		}
	}

	// Validate logical volumes
	lvNamesInVG := make(map[string]map[string]bool)

	for i, lv := range s.LogicalVolumes {
		if lv.Name == "" {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("logical volume at index %d: name is required", i))

			continue
		}

		if lv.VolumeGroup == "" {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("logical volume %q: volume group is required", lv.Name))
		} else if len(vgNames) > 0 {
			// Only validate VG reference if VGs are defined in this config
			if _, exists := vgNames[lv.VolumeGroup]; !exists {
				validationErrors = errors.Join(validationErrors, fmt.Errorf("logical volume %q: volume group %q not found in config", lv.Name, lv.VolumeGroup))
			}
		}

		// Check for duplicate LV names in same VG
		if lv.VolumeGroup != "" {
			if lvNamesInVG[lv.VolumeGroup] == nil {
				lvNamesInVG[lv.VolumeGroup] = make(map[string]bool)
			}

			if lvNamesInVG[lv.VolumeGroup][lv.Name] {
				validationErrors = errors.Join(validationErrors, fmt.Errorf("logical volume %q: duplicate name in volume group %q", lv.Name, lv.VolumeGroup))
			}

			lvNamesInVG[lv.VolumeGroup][lv.Name] = true
		}

		if lv.Size.IsZero() || lv.Size.Value() == 0 {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("logical volume %q: size must be greater than 0", lv.Name))
		}

		// Validate mount
		if lv.Mount != nil {
			if lv.Filesystem == nil {
				validationErrors = errors.Join(validationErrors, fmt.Errorf("logical volume %q: mount requires filesystem to be configured", lv.Name))
			}

			if lv.Mount.Path == "" {
				validationErrors = errors.Join(validationErrors, fmt.Errorf("logical volume %q: mount path is required", lv.Name))
			} else if !strings.HasPrefix(lv.Mount.Path, "/") {
				validationErrors = errors.Join(validationErrors, fmt.Errorf("logical volume %q: mount path must be absolute", lv.Name))
			} else if strings.Contains(lv.Mount.Path, "..") {
				validationErrors = errors.Join(validationErrors, fmt.Errorf("logical volume %q: mount path cannot contain '..'", lv.Name))
			}
		}

		// Validate encryption
		if lv.Encryption != nil {
			extraWarnings, extraErrors := lv.Encryption.Validate()
			warnings = append(warnings, extraWarnings...)
			validationErrors = errors.Join(validationErrors, extraErrors)
		}
	}

	return warnings, validationErrors
}
