// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pci

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//docgen:jsonschema

// RebindConfig defines the PCIRebind configuration name.
const RebindConfig = "PCIRebindConfig"

func init() {
	registry.Register(RebindConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &RebindConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.PCIRebindConfig = &RebindConfigV1Alpha1{}
	_ config.NamedDocument   = &RebindConfigV1Alpha1{}
)

// RebindConfigV1Alpha1 allows to configure PCI driver rebinds.
//
//	examples:
//	  - value: exampleRebindConfigAlpha1()
//	alias: PCIRebindConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/PCIRebindConfig
type RebindConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`
	//   description: |
	//     Name of the config document.
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     PCI device vendor and device ID.
	//   schemaRequired: true
	PCIVendorDeviceID string `yaml:"vendorDeviceID"`
	//   description: |
	//     Target driver to rebind the PCI device to.
	//   schemaRequired: true
	PCITargetDriver string `yaml:"targetDriver"`
}

// NewRebindConfigV1Alpha1 creates a new PCIRebindConfig config document.
func NewRebindConfigV1Alpha1() *RebindConfigV1Alpha1 {
	return &RebindConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       RebindConfig,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleRebindConfigAlpha1() *RebindConfigV1Alpha1 {
	cfg := NewRebindConfigV1Alpha1()
	cfg.MetaName = "ixgbe"
	cfg.PCIVendorDeviceID = "0000:04:00.00"
	cfg.PCITargetDriver = "vfio-pci"

	return cfg
}

// Clone implements config.Document interface.
func (s *RebindConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.Document interface.
func (s *RebindConfigV1Alpha1) Name() string {
	return s.MetaName
}

// PCIRebindConfigs implements config.PCIRebindConfig interface.
func (s *RebindConfigV1Alpha1) PCIRebindConfigs() []config.PCIRebindConfigDriver {
	return []config.PCIRebindConfigDriver{s}
}

// VendorDeviceID implements config.PCIRebindConfigDriver interface.
func (s *RebindConfigV1Alpha1) VendorDeviceID() string {
	return s.PCIVendorDeviceID
}

// TargetDriver implements config.PCIRebindConfigDriver interface.
func (s *RebindConfigV1Alpha1) TargetDriver() string {
	return s.PCITargetDriver
}

// RuntimeValidate implements config.RuntimeValidatable interface.
func (s *RebindConfigV1Alpha1) RuntimeValidate(ctx context.Context, st state.State, v validation.RuntimeMode, opts ...validation.Option) ([]string, error) {
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
		return []string{"IOMMU is not enabled"}, fmt.Errorf("IOMMU is not enabled")
	}

	return nil, nil
}
