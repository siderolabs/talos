// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//docgen:jsonschema

import (
	"errors"
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/labels"
)

// SELinuxPolicyConfigKind is a config document kind.
const SELinuxPolicyConfigKind = "SELinuxPolicyConfig"

func init() {
	registry.Register(SELinuxPolicyConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &SELinuxPolicyConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.SELinuxPolicyConfig = &SELinuxPolicyConfigV1Alpha1{}
	_ config.NamedDocument       = &SELinuxPolicyConfigV1Alpha1{}
	_ config.Validator           = &SELinuxPolicyConfigV1Alpha1{}
)

// SELinuxPolicyConfigV1Alpha1 is a SELinux policy module document.
//
//	examples:
//	  - value: exampleSELinuxPolicyConfigV1Alpha1()
//	alias: SELinuxPolicyConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/SELinuxPolicyConfig
type SELinuxPolicyConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`
	//   description: |
	//     Name of the policy module.
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Policy module in CIL, compiled with the Talos policy and loaded without a reboot.
	//     A module declares a type and calls a macro of the base policy, whose sources are in `/usr/share/selinux/talos`:
	//     `pod_domain` gives the rights of `pod_t`, `pod_privileged_domain` those of `pod_privileged_t`, and
	//     `pod_hostmon_domain` those of a process monitor, which reads `/proc` and the files of every pod and writes none.
	//     Whatever a module grants, a workload domain never reads STATE, connects to machined or ptraces a host service:
	//     `secilc` rejects such a module, which is left out while the other modules are loaded.
	//     A workload selects the type with `securityContext.seLinuxOptions.type`, which the CRI applies when the
	//     `SecurityProfileConfig` document sets `selinuxCategories`: load the module before the workload starts, and
	//     remove the workload before the module. A privileged container cannot select a type, it runs as
	//     `pod_privileged_t`, which `spc_t` is an alias of.
	//   schemaRequired: true
	PolicyContent string `yaml:"content"`
}

// NewSELinuxPolicyConfigV1Alpha1 creates a new SELinuxPolicyConfig document.
func NewSELinuxPolicyConfigV1Alpha1(name string) *SELinuxPolicyConfigV1Alpha1 {
	return &SELinuxPolicyConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       SELinuxPolicyConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleSELinuxPolicyConfigV1Alpha1() *SELinuxPolicyConfigV1Alpha1 {
	cfg := NewSELinuxPolicyConfigV1Alpha1("hostmon")
	cfg.PolicyContent = "(type pod_hostmon_t)\n(call pod_hostmon_domain (pod_hostmon_t))\n"

	return cfg
}

// Clone implements config.Document interface.
func (s *SELinuxPolicyConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *SELinuxPolicyConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Content implements config.SELinuxPolicyConfig interface.
func (s *SELinuxPolicyConfigV1Alpha1) Content() string {
	return s.PolicyContent
}

// SELinuxPolicyConfigSignal implements config.SELinuxPolicyConfig interface.
func (s *SELinuxPolicyConfigV1Alpha1) SELinuxPolicyConfigSignal() {}

// Validate implements config.Validator interface.
func (s *SELinuxPolicyConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var validationErrors error

	if err := labels.ValidateDNS1123Subdomain(s.MetaName); err != nil {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("invalid name %q: lowercase letters, digits, '-' and '.' only, as a DNS subdomain: %w", s.MetaName, err))
	}

	if s.PolicyContent == "" {
		validationErrors = errors.Join(validationErrors, errors.New("content is required"))
	}

	return nil, validationErrors
}
