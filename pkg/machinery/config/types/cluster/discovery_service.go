// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/siderolabs/gen/ensure"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// DiscoveryServiceKind is a discovery service config document kind.
const DiscoveryServiceKind = "DiscoveryServiceConfig"

func init() {
	registry.Register(DiscoveryServiceKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &DiscoveryServiceConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.DiscoveryServiceConfig       = &DiscoveryServiceConfigV1Alpha1{}
	_ config.NamedDocument                = &DiscoveryServiceConfigV1Alpha1{}
	_ config.Validator                    = &DiscoveryServiceConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &DiscoveryServiceConfigV1Alpha1{}
)

// DiscoveryServiceConfigV1Alpha1 is a config document to configure a discovery service.
//
//	examples:
//	  - value: exampleDiscoveryServiceConfigV1Alpha1()
//	alias: DiscoveryServiceConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/DiscoveryServiceConfig
type DiscoveryServiceConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the discovery service configuration.
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Discovery service endpoint to use.
	//   examples:
	//     - value: >
	//        "https://discovery.talos.dev/"
	//   schemaRequired: true
	//   schema:
	//     type: string
	//     pattern: "^(http|https|grpc)://"
	EndpointURL meta.URL `yaml:"endpoint"`
}

// NewDiscoveryServiceConfigV1Alpha1 creates a new discovery service config document.
func NewDiscoveryServiceConfigV1Alpha1(name string, endpoint *url.URL) *DiscoveryServiceConfigV1Alpha1 {
	return &DiscoveryServiceConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       DiscoveryServiceKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
		EndpointURL: meta.URL{
			URL: endpoint,
		},
	}
}

func exampleDiscoveryServiceConfigV1Alpha1() *DiscoveryServiceConfigV1Alpha1 {
	cfg := NewDiscoveryServiceConfigV1Alpha1("primary", ensure.Value(url.Parse("https://discovery.talos.dev/")))

	return cfg
}

// Clone implements config.Document interface.
func (s *DiscoveryServiceConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *DiscoveryServiceConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Endpoint implements config.DiscoveryServiceConfig interface.
func (s *DiscoveryServiceConfigV1Alpha1) Endpoint() *url.URL {
	if s == nil {
		return nil
	}

	return s.EndpointURL.URL
}

// Validate implements config.Validator interface.
func (s *DiscoveryServiceConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if s.MetaName == "" {
		return nil, errors.New("name is required")
	}

	if s.EndpointURL.URL == nil {
		return nil, errors.New("endpoint is required")
	}

	if err := ValidateDiscoveryServiceEndpoint(s.EndpointURL.URL); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateDiscoveryServiceEndpoint validates the discovery service endpoint URL.
func ValidateDiscoveryServiceEndpoint(endpoint *url.URL) error {
	if endpoint.Scheme == "" {
		return fmt.Errorf("endpoint scheme is required")
	}

	switch endpoint.Scheme {
	case "http", "https", "grpc":
	default:
		return fmt.Errorf("endpoint scheme must be http://, https:// or grpc://")
	}

	host := endpoint.Hostname()
	if host == "" {
		return fmt.Errorf("endpoint host is required")
	}

	return nil
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
//
// A discovery service document conflicts with the deprecated v1alpha1 cluster discovery config:
// the two are mutually exclusive. The mere presence of the .cluster.discovery block conflicts,
// regardless of its contents.
func (s *DiscoveryServiceConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil && v1alpha1Cfg.ClusterConfig.ClusterDiscoveryConfig != nil { //nolint:staticcheck // checking presence of legacy config
		return errors.New("discovery service is already configured in .cluster.discovery of the v1alpha1 config")
	}

	return nil
}
