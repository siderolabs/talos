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

// SysctlConfigKind is a sysctl config document kind.
const SysctlConfigKind = "SysctlConfig"

func init() {
	registry.Register(SysctlConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &SysctlConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.SysctlConfig = &SysctlConfigV1Alpha1{}
	_ config.Validator    = &SysctlConfigV1Alpha1{}
)

// SysctlConfigV1Alpha1 configures Linux kernel sysctl values.
//
//	examples:
//	  - value: exampleSysctlConfigV1Alpha1()
//	alias: SysctlConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/SysctlConfig
type SysctlConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Used to configure the machine's sysctls (kernel parameters under `/proc/sys`).
	//     Values from this document are merged with the deprecated v1alpha1 machine.sysctls values (if set),
	//     with this document taking precedence on key conflicts.
	//   examples:
	//     - name: SysctlConfig usage example.
	//       value: exampleSysctlParams()
	//   schema:
	//     type: object
	//     patternProperties:
	//       ".*":
	//         type: string
	Params map[string]string `yaml:"params"`
}

// NewSysctlConfigV1Alpha1 creates a new SysctlConfig config document.
func NewSysctlConfigV1Alpha1() *SysctlConfigV1Alpha1 {
	return &SysctlConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       SysctlConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleSysctlConfigV1Alpha1() *SysctlConfigV1Alpha1 {
	cfg := NewSysctlConfigV1Alpha1()
	cfg.Params = exampleSysctlParams()

	return cfg
}

func exampleSysctlParams() map[string]string {
	return map[string]string{
		"fs.inotify.max_user_watches":         "12288",
		"kernel.domainname":                   "talos.dev",
		"net.ipv4.ip_forward":                 "0",
		"net/ipv6/conf/eth0.100/disable_ipv6": "1",
	}
}

// Clone implements config.Document interface.
func (s *SysctlConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Sysctls implements config.SysctlConfig interface.
func (s *SysctlConfigV1Alpha1) Sysctls() map[string]string {
	return s.Params
}

// Validate implements config.Validator interface.
func (s *SysctlConfigV1Alpha1) Validate(
	validation.RuntimeMode,
	...validation.Option,
) ([]string, error) {
	var err error

	for key := range s.Params {
		if key == "" {
			err = errors.Join(err, errors.New("sysctl key cannot be empty"))
		}
	}

	return nil, err
}
