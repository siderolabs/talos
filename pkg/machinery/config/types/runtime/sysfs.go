// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//docgen:jsonschema

import (
	"errors"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// SysfsConfigKind is a sysfs config document kind.
const SysfsConfigKind = "SysfsConfig"

func init() {
	registry.Register(SysfsConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &SysfsConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.SysfsConfig = &SysfsConfigV1Alpha1{}
	_ config.Validator   = &SysfsConfigV1Alpha1{}
)

// SysfsConfigV1Alpha1 configures Linux kernel sysfs values.
//
//	examples:
//	  - value: exampleSysfsConfigV1Alpha1()
//	alias: SysfsConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/SysfsConfig
type SysfsConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Used to configure the machine's sysfs (kernel attributes under `/sys`).
	//     Values from this document are merged with the deprecated v1alpha1 machine.sysfs values (if set),
	//     with this document taking precedence on key conflicts.
	//   examples:
	//     - name: SysfsConfig usage example.
	//       value: exampleSysfsParams()
	//   schema:
	//     type: object
	//     patternProperties:
	//       ".*":
	//         type: string
	Params map[string]string `yaml:"params"`
}

// NewSysfsConfigV1Alpha1 creates a new SysfsConfig config document.
func NewSysfsConfigV1Alpha1() *SysfsConfigV1Alpha1 {
	return &SysfsConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       SysfsConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleSysfsConfigV1Alpha1() *SysfsConfigV1Alpha1 {
	cfg := NewSysfsConfigV1Alpha1()
	cfg.Params = exampleSysfsParams()

	return cfg
}

func exampleSysfsParams() map[string]string {
	return map[string]string{
		"devices.system.cpu.cpu0.cpufreq.scaling_governor": "performance",
	}
}

// Clone implements config.Document interface.
func (s *SysfsConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Sysfs implements config.SysfsConfig interface.
func (s *SysfsConfigV1Alpha1) Sysfs() map[string]string {
	return s.Params
}

// Validate implements config.Validator interface.
func (s *SysfsConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var err error

	for key := range s.Params {
		if key == "" {
			err = errors.Join(err, errors.New("sysfs key cannot be empty"))
		}
	}

	return nil, err
}
