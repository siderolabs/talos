// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"context"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//docgen:jsonschema

// PCIDriverRebindConfig defines the PCIDriverRebind configuration name.
const PCIDriverRebindConfig = "PCIDriverRebindConfig"

func init() {
	registry.Register(PCIDriverRebindConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &PCIDriverRebindConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.PCIDriverRebindConfig = &PCIDriverRebindConfigV1Alpha1{}
	_ config.NamedDocument         = &PCIDriverRebindConfigV1Alpha1{}
)

// PCIDriverRebindConfigV1Alpha1 allows to configure PCI driver rebinds.
//
//	examples:
//	  - value: examplePCIDriverRebindConfigAlpha1()
//	alias: PCIDriverRebindConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/PCIDriverRebindConfig
type PCIDriverRebindConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`
	//   description: |
	//     PCI device id
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Target driver to rebind the PCI device to.
	//   schemaRequired: true
	PCITargetDriver string `yaml:"targetDriver"`
}

// NewPCIDriverRebindConfigV1Alpha1 creates a new PCIDriverRebindConfig config document.
func NewPCIDriverRebindConfigV1Alpha1() *PCIDriverRebindConfigV1Alpha1 {
	return &PCIDriverRebindConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       PCIDriverRebindConfig,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func examplePCIDriverRebindConfigAlpha1() *PCIDriverRebindConfigV1Alpha1 {
	cfg := NewPCIDriverRebindConfigV1Alpha1()
	cfg.MetaName = "0000:04:00.00"
	cfg.PCITargetDriver = "vfio-pci"

	return cfg
}

// Clone implements config.Document interface.
func (s *PCIDriverRebindConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.Document interface.
func (s *PCIDriverRebindConfigV1Alpha1) Name() string {
	return s.MetaName
}

// PCIDriverRebindConfigs implements config.PCIDriverRebindConfig interface.
func (s *PCIDriverRebindConfigV1Alpha1) PCIDriverRebindConfigs() []config.PCIDriverRebindConfigDriver {
	return []config.PCIDriverRebindConfigDriver{s}
}

// PCIID implements config.PCIDriverRebindConfigDriver interface.
func (s *PCIDriverRebindConfigV1Alpha1) PCIID() string {
	return s.MetaName
}

// TargetDriver implements config.PCIDriverRebindConfigDriver interface.
func (s *PCIDriverRebindConfigV1Alpha1) TargetDriver() string {
	return s.PCITargetDriver
}

// RuntimeValidate implements config.RuntimeValidatable interface.
func (s *PCIDriverRebindConfigV1Alpha1) RuntimeValidate(ctx context.Context, st state.State, v validation.RuntimeMode, opts ...validation.Option) ([]string, error) {
	var count int

	if err := filepath.WalkDir("/sys/class/iommu", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel("/sys/class/iommu", path)
		if err != nil {
			return err
		}

		if rel == "." {
			return nil
		}

		count++

		return nil
	}); err != nil {
		return nil, err
	}

	if count == 0 {
		return []string{"IOMMU is not enabled, this config change might not have any effect"}, nil
	}

	return nil, nil
}
