// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//docgen:jsonschema

import (
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// SecurityProfileConfigKind is a config document kind.
const SecurityProfileConfigKind = "SecurityProfileConfig"

func init() {
	registry.Register(SecurityProfileConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &SecurityProfileConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.SecurityProfileConfig = &SecurityProfileConfigV1Alpha1{}
	_ config.Validator             = &SecurityProfileConfigV1Alpha1{}
)

// SecurityProfileConfigV1Alpha1 is a node security profile configuration document.
//
//	description: |
//	  The security profile groups node-level security hardening features. Additional hardening options
//	  will be added to this document over time.
//
//	  Currently it controls workload isolation: running the container runtime plane (CRI containerd, the
//	  kubelet, and all pods) inside a dedicated PID and mount namespace anchored by the `sandboxd` service,
//	  isolating them from `machined` (PID 1) and its file descriptors.
//
//	  `talosctl gen config` emits this document with `workloadIsolation: true` for Talos 1.14+, so new
//	  clusters are isolated by default; clusters upgraded from older versions do not have the document and
//	  keep the old (non-isolated) behavior unless it is added.
//
//	  Note: with workload isolation enabled, the deprecated in-tree Kubernetes iSCSI volume plugin does not
//	  work (the kubelet cannot reach the host iscsid across the sandbox); use a CSI driver instead.
//	examples:
//	  - value: exampleSecurityProfileConfigV1Alpha1()
//	alias: SecurityProfileConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/SecurityProfileConfig
type SecurityProfileConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Enable workload isolation (run the container plane inside the sandbox namespace).
	//   schema:
	//     type: boolean
	WorkloadIsolationEnabled *bool `yaml:"workloadIsolation,omitempty"`
}

// NewSecurityProfileConfigV1Alpha1 creates a new security profile config document.
func NewSecurityProfileConfigV1Alpha1() *SecurityProfileConfigV1Alpha1 {
	return &SecurityProfileConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       SecurityProfileConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleSecurityProfileConfigV1Alpha1() *SecurityProfileConfigV1Alpha1 {
	cfg := NewSecurityProfileConfigV1Alpha1()
	cfg.WorkloadIsolationEnabled = new(true)

	return cfg
}

// Clone implements config.Document interface.
func (s *SecurityProfileConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *SecurityProfileConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	return nil, nil
}

// SecurityProfileConfigSignal is a signal for security profile config.
func (s *SecurityProfileConfigV1Alpha1) SecurityProfileConfigSignal() {}

// WorkloadIsolation implements config.SecurityProfileConfig interface.
func (s *SecurityProfileConfigV1Alpha1) WorkloadIsolation() bool {
	return pointer.SafeDeref(s.WorkloadIsolationEnabled)
}
