// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//docgen:jsonschema

// CRIBaseRuntimeSpecConfigKind is the CRIBaseRuntimeSpecConfig configuration document kind.
const CRIBaseRuntimeSpecConfigKind = "CRIBaseRuntimeSpecConfig"

func init() {
	registry.Register(CRIBaseRuntimeSpecConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &CRIBaseRuntimeSpecConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.CRIBaseRuntimeSpecConfig     = &CRIBaseRuntimeSpecConfigV1Alpha1{}
	_ config.Validator                    = &CRIBaseRuntimeSpecConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &CRIBaseRuntimeSpecConfigV1Alpha1{}
)

// CRIBaseRuntimeSpecConfigV1Alpha1 configures the base OCI runtime specification for CRI containers.
//
//	examples:
//	  - value: exampleCRIBaseRuntimeSpecConfigV1Alpha1()
//	alias: CRIBaseRuntimeSpecConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/CRIBaseRuntimeSpecConfig
type CRIBaseRuntimeSpecConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Overrides for the default OCI runtime specification used by CRI containers.
	//
	//     This document is mutually exclusive with the deprecated
	//     `.machine.baseRuntimeSpecOverrides` field.
	//
	//     Strategic merge patches replace this overrides object as a whole, so
	//     reapplying the same document is idempotent.
	//
	//     Applying, updating, or removing these overrides restarts CRI automatically.
	//     A machine reboot is not required.
	//   schema:
	//     type: object
	OverridesConfig meta.Unstructured `yaml:"overrides,omitempty" merge:"replace"`
}

// NewCRIBaseRuntimeSpecConfigV1Alpha1 creates a new CRIBaseRuntimeSpecConfig document.
func NewCRIBaseRuntimeSpecConfigV1Alpha1() *CRIBaseRuntimeSpecConfigV1Alpha1 {
	return &CRIBaseRuntimeSpecConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       CRIBaseRuntimeSpecConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleCRIBaseRuntimeSpecConfigV1Alpha1() *CRIBaseRuntimeSpecConfigV1Alpha1 {
	cfg := NewCRIBaseRuntimeSpecConfigV1Alpha1()
	cfg.OverridesConfig.Object = map[string]any{
		"process": map[string]any{
			"rlimits": []any{
				map[string]any{
					"type": "RLIMIT_NOFILE",
					"hard": 1024,
					"soft": 1024,
				},
			},
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (cfg *CRIBaseRuntimeSpecConfigV1Alpha1) Clone() config.Document {
	return cfg.DeepCopy()
}

// CRIBaseRuntimeSpecConfigSignal implements config.CRIBaseRuntimeSpecConfig interface.
func (cfg *CRIBaseRuntimeSpecConfigV1Alpha1) CRIBaseRuntimeSpecConfigSignal() {}

// Overrides implements config.CRIBaseRuntimeSpecConfig interface.
func (cfg *CRIBaseRuntimeSpecConfigV1Alpha1) Overrides() map[string]any {
	return cfg.OverridesConfig.Object
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (cfg *CRIBaseRuntimeSpecConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg != nil && v1alpha1Cfg.MachineConfig != nil && v1alpha1Cfg.MachineConfig.MachineBaseRuntimeSpecOverrides.Object != nil { //nolint:staticcheck // check deprecated configuration
		return errors.New("base runtime spec overrides are already set in v1alpha1 config")
	}

	return nil
}

// Validate implements config.Validator interface.
func (cfg *CRIBaseRuntimeSpecConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	jsonSpec, err := json.Marshal(cfg.Overrides())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal base runtime spec overrides: %w", err)
	}

	var ociSpec specs.Spec

	if err = json.Unmarshal(jsonSpec, &ociSpec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal base runtime spec overrides: %w", err)
	}

	return nil, nil
}
