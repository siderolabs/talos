// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//docgen:jsonschema

import (
	"errors"
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// UnattendedInstallConfigKind is an UnattendedInstallConfig config document kind.
const UnattendedInstallConfigKind = "UnattendedInstallConfig"

func init() {
	registry.Register(UnattendedInstallConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &UnattendedInstallConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.UnattendedInstallConfig      = &UnattendedInstallConfigV1Alpha1{}
	_ config.Validator                    = &UnattendedInstallConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &UnattendedInstallConfigV1Alpha1{}
)

// UnattendedInstallConfigV1Alpha1 is an UnattendedInstallConfig config document.
//
//	examples:
//	  - value: exampleUnattendedInstallConfigV1Alpha1()
//	alias: UnattendedInstallConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/UnattendedInstall
type UnattendedInstallConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	// Reboot is a flag to indicate if the system should reboot after installation.
	// If not set, Talos will reboot only if the installer.image is set.
	Reboot *bool `yaml:"reboot,omitempty"`

	//   description: |
	//     The installer describes the source of the installation.
	//   examples:
	//     - value: exampleInstallerSpec()
	Installer InstallerSpec `yaml:"installer"`

	//   description: |
	//     The provisioning describes how the installation disk should be provisioned.
	ProvisioningSpec ProvisioningSpec `yaml:"provisioning"`
}

// InstallerSpec describes the installer to perform the installation.
type InstallerSpec struct {
	//   description: |
	//     Allows for supplying the image used to perform the installation.
	//     Image reference for each Talos release can be found on
	//     [GitHub releases page](https://github.com/siderolabs/talos/releases).
	//
	//     If not set, it will run installer based on the current Talos version
	//     and current schematic (this requires booting asset built by Image
	//     Factory).
	//   examples:
	//     - value: '"factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:latest"'
	Image string `yaml:"image,omitempty"`
}

func exampleInstallerSpec() InstallerSpec {
	return InstallerSpec{
		Image: "factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:latest",
	}
}

// ProvisioningSpec describes how the Physical Volumes are provisioned.
type ProvisioningSpec struct {
	//   description: |
	//     Matches disks to initialize as physical volumes.
	DiskSelector DiskSelectorSpec `yaml:"diskSelector,omitempty"`

	//   description: |
	//     Indicates if the installation disk should be wiped at installation time.
	//     Defaults to `true`.
	//   values:
	//     - true
	//     - yes
	//     - false
	//     - no
	Wipe *bool `yaml:"wipe,omitempty"`
}

// IsZero reports whether the spec is empty.
func (s ProvisioningSpec) IsZero() bool {
	return s.DiskSelector.IsZero()
}

// Validate parses selector without mutating stored config.
func (s ProvisioningSpec) Validate() error {
	if s.DiskSelector.Match.IsZero() {
		return errors.New("provisioning.volumeSelector.match is required")
	}

	if err := s.DiskSelector.Match.ParseBool(celenv.DiskLocator()); err != nil {
		return fmt.Errorf("provisioning.volumeSelector.match: %w", err)
	}

	return nil
}

// DiskSelectorSpec matches disks with CEL.
type DiskSelectorSpec struct {
	//   description: |
	//     CEL expression matching a disk.
	//   schema:
	//     type: string
	//   examples:
	//     - value: >
	//        exampleDiskSelector()
	//       name: match raw volume partitions labeled r-lvm*
	Match cel.Expression `yaml:"match,omitempty"`
}

// IsZero reports whether the selector is empty.
func (s DiskSelectorSpec) IsZero() bool {
	return s.Match.IsZero()
}

func exampleUnattendedInstallConfigV1Alpha1() *UnattendedInstallConfigV1Alpha1 {
	cfg := NewUnattendedInstallConfigV1Alpha1()
	cfg.Installer = exampleInstallerSpec()
	cfg.ProvisioningSpec.DiskSelector.Match = exampleDiskSelector()
	cfg.ProvisioningSpec.Wipe = new(true)

	return cfg
}

func exampleDiskSelector() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`disk.dev_path == "/dev/sda"`, celenv.VolumeLocator()))
}

// NewUnattendedInstallConfigV1Alpha1 creates a new UnattendedInstallConfig config document.
func NewUnattendedInstallConfigV1Alpha1() *UnattendedInstallConfigV1Alpha1 {
	return &UnattendedInstallConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       UnattendedInstallConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

// Clone implements config.Document interface.
func (s *UnattendedInstallConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *UnattendedInstallConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		warnings         []string
		validationErrors error
	)

	if s.Installer.Image == "" {
		warnings = append(warnings, "installer.image is not set, if Talos is not booted from asset built by Image Factory, installation will fail")
	}

	if err := s.ProvisioningSpec.Validate(); err != nil {
		validationErrors = errors.Join(validationErrors, err)
	}

	return warnings, validationErrors
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *UnattendedInstallConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.MachineConfig != nil && v1alpha1Cfg.MachineConfig.MachineInstall != nil { //nolint:staticcheck // testing deprecated field
		return errors.New("UnattendedInstallConfig config is incompatible with v1alpha1 config (.machine.install)")
	}

	return nil
}

// UnattendedInstallConfigSignal implements config.UnattendedInstallConfig interface.
func (s *UnattendedInstallConfigV1Alpha1) UnattendedInstallConfigSignal() {}

// InstallerImage implements config.UnattendedInstallConfig interface.
func (s *UnattendedInstallConfigV1Alpha1) InstallerImage() string {
	return s.Installer.Image
}

// VolumeSelector implements config.UnattendedInstallConfig interface.
func (s *UnattendedInstallConfigV1Alpha1) VolumeSelector() cel.Expression {
	return s.ProvisioningSpec.DiskSelector.Match
}

// RebootAfterInstall implements config.UnattendedInstallConfig interface.
func (s *UnattendedInstallConfigV1Alpha1) RebootAfterInstall() *bool {
	return s.Reboot
}

// VolumeWipe implements config.UnattendedInstallConfig interface.
func (s *UnattendedInstallConfigV1Alpha1) VolumeWipe() bool {
	if s.ProvisioningSpec.Wipe == nil {
		return true
	}

	return *s.ProvisioningSpec.Wipe
}
