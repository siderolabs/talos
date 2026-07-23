// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/siderolabs/gen/ensure"
	sideronet "github.com/siderolabs/net"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//docgen:jsonschema

// KubeClusterConfig defines the KubeClusterConfig configuration name.
const KubeClusterConfig = "KubeClusterConfig"

func init() {
	registry.Register(KubeClusterConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeClusterConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sClusterConfig             = &KubeClusterConfigV1Alpha1{}
	_ config.Validator                    = &KubeClusterConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeClusterConfigV1Alpha1{}
)

// KubeClusterConfigV1Alpha1 configures Kubernetes cluster base settings.
//
//	examples:
//	  - value: exampleKubeClusterConfig1V1Alpha1()
//	alias: KubeClusterConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeClusterConfig
type KubeClusterConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     The cluster name.
	//     It is used mostly for informational purposes, and gets included into kubeconfig.
	//   schemaRequired: true
	ClusterNameConfig string `yaml:"clusterName"`
	//   description: |
	//     The Kubernetes API endpoint.
	//     For a single-node cluster, this can be the same as the node's IP address.
	//     For a multi-node cluster, this should be the load balancer's IP address or DNS name,
	//     or any other address (VIP, BGP, etc.) that can be used to reach the Kubernetes API server from the nodes.
	//   schema:
	//     type: string
	//     pattern: "^https://"
	//   schemaRequired: true
	ClusterEndpointConfig meta.URL `yaml:"endpoint"`
}

// NewKubeClusterConfigV1Alpha1 creates a new KubeClusterConfig config document.
func NewKubeClusterConfigV1Alpha1() *KubeClusterConfigV1Alpha1 {
	return &KubeClusterConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeClusterConfig,
		},
	}
}

func exampleKubeClusterConfig1V1Alpha1() *KubeClusterConfigV1Alpha1 {
	cfg := NewKubeClusterConfigV1Alpha1()
	cfg.ClusterNameConfig = "example-cluster"
	cfg.ClusterEndpointConfig = meta.URL{URL: ensure.Value(url.Parse("https://example.com:6443/"))}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeClusterConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *KubeClusterConfigV1Alpha1) Validate(_ validation.RuntimeMode, _ ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.ClusterNameConfig == "" {
		errs = errors.Join(errs, fmt.Errorf("clusterName must be specified"))
	}

	if s.ClusterEndpointConfig == (meta.URL{}) {
		errs = errors.Join(errs, fmt.Errorf("endpoint must be specified"))
	} else if err := sideronet.ValidateEndpointURI(s.ClusterEndpointConfig.URL.String()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("cluster endpoint is invalid: %w", err))
	}

	return warnings, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeClusterConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil && v1alpha1Cfg.ClusterConfig.ClusterName != "" { //nolint:staticcheck // legacy access
		return errors.New("cluster name is already set in the v1alpha1 config (.cluster.clusterName). Please remove it and use only the new KubeClusterConfig document to avoid conflicts")
	}

	if v1alpha1Cfg.ClusterConfig != nil && v1alpha1Cfg.ClusterConfig.ControlPlane != nil && //nolint:staticcheck // legacy access
		v1alpha1Cfg.ClusterConfig.ControlPlane.Endpoint != nil { //nolint:staticcheck // legacy access
		return errors.New("cluster endpoint is already set in the v1alpha1 config (.cluster.controlPlane.endpoint). " +
			"Please remove it and use only the new KubeClusterConfig document to avoid conflicts")
	}

	return nil
}

// ClusterName implements config.K8sClusterConfig interface.
func (s *KubeClusterConfigV1Alpha1) ClusterName() string {
	return s.ClusterNameConfig
}

// ClusterEndpoint implements config.K8sClusterConfig interface.
func (s *KubeClusterConfigV1Alpha1) ClusterEndpoint() *url.URL {
	return s.ClusterEndpointConfig.URL
}
