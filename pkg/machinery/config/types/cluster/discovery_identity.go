// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

//docgen:jsonschema

import (
	"encoding/base64"
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

// DiscoveryIdentityKind is a discovery identity config document kind.
const (
	DiscoveryIdentityKind = "DiscoveryIdentityConfig"
)

var (
	ClusterIDEncoding     = base64.StdEncoding
	ClusterSecretEncoding = base64.StdEncoding
)

func init() {
	registry.Register(DiscoveryIdentityKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &DiscoveryIdentityConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.DiscoveryIdentityConfig      = &DiscoveryIdentityConfigV1Alpha1{}
	_ config.Validator                    = &DiscoveryIdentityConfigV1Alpha1{}
	_ config.SecretDocument               = &DiscoveryIdentityConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &DiscoveryIdentityConfigV1Alpha1{}
)

// DiscoveryIdentityConfigV1Alpha1 is a config document to configure the cluster identity used by the discovery service.
//
//	examples:
//	  - value: exampleDiscoveryIdentityConfigV1Alpha1()
//	alias: DiscoveryIdentityConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/DiscoveryIdentityConfig
type DiscoveryIdentityConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Globally unique identifier for this cluster (base64 encoded random 32 bytes).
	//   schemaRequired: true
	MetaClusterID string `yaml:"clusterID"`
	//   description: |
	//     Shared secret of cluster (base64 encoded random 32 bytes).
	//     This secret is shared among cluster members but should never be sent over the network.
	//   schemaRequired: true
	MetaClusterSecret string `yaml:"clusterSecret"`
}

// NewDiscoveryIdentityConfigV1Alpha1 creates a new discovery identity config document.
func NewDiscoveryIdentityConfigV1Alpha1(clusterID, clusterSecret string) *DiscoveryIdentityConfigV1Alpha1 {
	return &DiscoveryIdentityConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       DiscoveryIdentityKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaClusterID:     clusterID,
		MetaClusterSecret: clusterSecret,
	}
}

func exampleDiscoveryIdentityConfigV1Alpha1() *DiscoveryIdentityConfigV1Alpha1 {
	return NewDiscoveryIdentityConfigV1Alpha1(
		"cluster-id-base64-encoded-32-bytes",
		"cluster-secret-base64-encoded-32-bytes",
	)
}

// Clone implements config.Document interface.
func (s *DiscoveryIdentityConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// ClusterID implements config.DiscoveryIdentityConfig interface.
func (s *DiscoveryIdentityConfigV1Alpha1) ClusterID() string {
	if s == nil {
		return ""
	}

	return s.MetaClusterID
}

// ClusterSecret implements config.DiscoveryIdentityConfig interface.
func (s *DiscoveryIdentityConfigV1Alpha1) ClusterSecret() string {
	if s == nil {
		return ""
	}

	return s.MetaClusterSecret
}

// Validate implements config.Validator interface.
func (s *DiscoveryIdentityConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	// We don't need to validate the clusterID is base64 encoded nor that it's 32 bytes long,
	// because we only use it as a unique identifier. We never need to decode it.
	//
	// That said, `talosctl gen secrets` will generate a valid, random, 32-byte, base64-encoded cluster ID.
	// Before Talos 1.14, it used URLEncoding, while the rest of the codebase uses StdEncoding. The reason
	// is unknown, but we suspect it was just a mistake missed in the original implementation.
	// We've aligned the encoding since, and starting with Talos 1.14, `talosctl gen secrets`
	// will generate a StdEncoding clusterID.
	if s.MetaClusterID == "" {
		return nil, errors.New("clusterID is required")
	}

	// The cluster secret is used as an AES encryption key, so it must:
	// - be base64 encoded (via StdEncoding)
	// - decode to 32 bytes for AES-256
	if s.MetaClusterSecret == "" {
		return nil, errors.New("clusterSecret is required")
	}

	if err := ValidateBase64WithLen(s.MetaClusterSecret, ClusterSecretEncoding, constants.DefaultClusterSecretSize); err != nil {
		return nil, fmt.Errorf("invalid clusterSecret: %w", err)
	}

	return nil, nil
}

// Redact implements config.SecretDocument interface.
func (s *DiscoveryIdentityConfigV1Alpha1) Redact(replacement string) {
	if s.MetaClusterSecret != "" {
		s.MetaClusterSecret = replacement
	}
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
//
// The multi-doc DiscoveryIdentityConfig is mutually exclusive with the v1alpha1 cluster identity config.
func (s *DiscoveryIdentityConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil &&
		(v1alpha1Cfg.ClusterConfig.ClusterID != "" || v1alpha1Cfg.ClusterConfig.ClusterSecret != "") { //nolint:staticcheck // checking presence of legacy config
		return errors.New("cluster identity is already configured in .cluster.id/.cluster.secret of the v1alpha1 config")
	}

	return nil
}

// ValidateBase64WithLen validates that the given string is a valid base64 encoded
// string and that it decodes to the expected length in bytes.
func ValidateBase64WithLen(base64Str string, encoding *base64.Encoding, wantLenBytes int) error {
	decoded, err := encoding.DecodeString(base64Str)
	if err != nil {
		return fmt.Errorf("failed to decode from base64: %s; %w", base64Str, err)
	}

	if len(decoded) != wantLenBytes {
		return fmt.Errorf("expected %d bytes, got %d: %s", wantLenBytes, len(decoded), base64Str)
	}

	return nil
}
