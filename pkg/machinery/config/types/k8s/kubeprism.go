// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//docgen:jsonschema

// KubePrismConfig defines the KubePrismConfig configuration name.
const KubePrismConfig = "KubePrismConfig"

func init() {
	registry.Register(KubePrismConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubePrismConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sKubePrismConfig           = &KubePrismConfigV1Alpha1{}
	_ config.Validator                    = &KubePrismConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubePrismConfigV1Alpha1{}
)

// KubePrismConfigV1Alpha1 configures node-local Kubernetes API load balancer.
//
//	examples:
//	  - value: exampleKubePrismConfigV1Alpha1()
//	alias: KubePrismConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubePrismConfig
type KubePrismConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     KubePrism port.
	//     The load balancer will be started on `127.0.0.1:<port>` and it will
	//     automatically include a controlplane endpoint and direct addresses of
	//     all controlplane nodes in the cluster.
	//     The KubePrism will pick up the route(s) with the lowest RTT to the controlplane nodes,
	//     excluding the unavailable ones, and will automatically update the route list when the controlplane nodes change.
	//   schemaRequired: true
	PortConfig int `yaml:"port"`
	//   description: |
	//     Override the TLS server name (SNI) used by the kubelet when connecting to
	//     the KubePrism endpoint.
	//
	//     KubePrism still listens on `127.0.0.1:<port>` and the kubelet still dials
	//     that address, but the generated kubelet kubeconfig will carry
	//     `clusters[0].cluster.tls-server-name` set to this value, so the kubelet
	//     uses it for SNI and certificate hostname verification.
	//
	//     This is useful when KubePrism's upstream apiserver is reached through an
	//     SNI-routing L4 proxy (for example nginx-ingress in ssl-passthrough mode in
	//     front of a Kamaji-hosted apiserver), where SNI=127.0.0.1 doesn't match any
	//     route and the proxy serves a fallback certificate.
	//
	//     When empty (default), no `tls-server-name` is set and behavior is unchanged.
	TLSServerNameConfig string `yaml:"tlsServerName,omitempty"`
}

// NewKubePrismConfigV1Alpha1 creates a new KubePrismConfig config document.
func NewKubePrismConfigV1Alpha1() *KubePrismConfigV1Alpha1 {
	return &KubePrismConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubePrismConfig,
		},
	}
}

func exampleKubePrismConfigV1Alpha1() *KubePrismConfigV1Alpha1 {
	cfg := NewKubePrismConfigV1Alpha1()
	cfg.PortConfig = constants.DefaultKubePrismPort

	return cfg
}

// Clone implements config.Document interface.
func (s *KubePrismConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *KubePrismConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.PortConfig <= 0 || s.PortConfig > 65535 {
		errs = errors.Join(errs, fmt.Errorf("invalid port %d: must be in range 1-65535", s.PortConfig))
	}

	return warnings, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubePrismConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.MachineConfig != nil && v1alpha1Cfg.MachineConfig.MachineFeatures != nil { //nolint:staticcheck // legacy access
		if v1alpha1Cfg.MachineConfig.MachineFeatures.KubePrismSupport != nil { //nolint:staticcheck // legacy access
			return errors.New("KubePrism config in v1alpha1 config (.machine.features.kubePrism) can't be used with KubePrismConfig document, please remove it to avoid conflicts")
		}
	}

	return nil
}

// K8sKubePrismConfigSignal implements config.K8sKubePrismConfig interface.
func (s *KubePrismConfigV1Alpha1) K8sKubePrismConfigSignal() {}

// Port implements config.K8sKubePrismConfig interface.
func (s *KubePrismConfigV1Alpha1) Port() int {
	return s.PortConfig
}

// TLSServerName implements config.K8sKubePrismConfig interface.
func (s *KubePrismConfigV1Alpha1) TLSServerName() string {
	return s.TLSServerNameConfig
}
