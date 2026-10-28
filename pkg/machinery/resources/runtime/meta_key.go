// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	metaconsts "github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/redactable"
)

// MetaKeyType is type of MetaKey resource.
const MetaKeyType = resource.Type("MetaKeys.runtime.talos.dev")

// MetaKey resource holds value of a key in META partition.
type MetaKey = typed.Resource[MetaKeySpec, MetaKeyExtension]

// MetaKeySpec describes status of the defined sysctls.
//
//gotagsrewrite:gen
type MetaKeySpec struct {
	Value string `yaml:"value" protobuf:"1"`
}

// MetaKeyValueRedactor redacts the secrets held in the value of a single META key.
//
// It returns the value with the secrets taken out, and whether the value carried any secret at all.
// A value with nothing to redact is returned unchanged and reported as carrying no secrets, so that
// it is served in full and is not marked as redacted.
type MetaKeyValueRedactor func(value string) (redacted string, hasSecrets bool)

// sensitiveMetaKeys maps the META keys whose value might hold a secret to the way it is redacted.
//
// Sensitivity is a property of the resource type, but what a MetaKey holds depends entirely on which
// key it is: most of them carry upgrade markers, a machine UUID override or user-reserved data, all
// of which are safe to show. Only the keys listed here are ever redacted, so the resource stays useful
// once redacted.
//
// The default is to redact the whole value; a key which holds a marshaled structure with only a few
// sensitive fields in it refines this with [RegisterMetaKeyValueRedactor].
var sensitiveMetaKeys = map[resource.ID]MetaKeyValueRedactor{
	// the unique token identifying this node, written by Omni
	MetaKeyTagToID(metaconsts.UniqueMachineToken): redactWholeValue,
	// the platform network configuration of the metal platform: addresses, links and routes, which
	// are not secrets on their own, but the embedded Wireguard, VIP and probe specs might be.
	// Refined by the network resource package, see redactPlatformConfigValue.
	MetaKeyTagToID(metaconsts.MetalNetworkPlatformConfig): redactWholeValue,
	// the STATE partition encryption configuration
	MetaKeyTagToID(metaconsts.StateEncryptionConfig): redactWholeValue,
}

// RegisterMetaKeyValueRedactor refines the way the value of a single META key is redacted.
//
// It is meant to be called from an init function. The key has to be one of the keys already known to
// be sensitive, so that the list of those stays in a single place, and registering can only ever make
// the redaction more precise, never weaker than blanking the value out.
func RegisterMetaKeyValueRedactor(id resource.ID, redactor MetaKeyValueRedactor) {
	if _, sensitive := sensitiveMetaKeys[id]; !sensitive {
		panic(fmt.Sprintf("META key %q holds no secrets, nothing to redact", id))
	}

	sensitiveMetaKeys[id] = redactor
}

// redactWholeValue redacts the value of a META key which is a secret in its entirety.
func redactWholeValue(string) (string, bool) {
	return constants.Redacted, true
}

// Check interfaces.
var _ redactable.Spec = &MetaKeySpec{}

// HasSecrets implements redactable.Spec interface.
//
// This is hand-written rather than generated, because whether the value is a secret follows from the
// resource ID and not from the shape of the spec, which is all redactgen can see.
func (spec *MetaKeySpec) HasSecrets(md resource.Metadata) bool {
	_, hasSecrets := spec.redactValue(md)

	return hasSecrets
}

// RedactSecrets implements redactable.Spec interface.
func (spec *MetaKeySpec) RedactSecrets(md resource.Metadata) {
	if redacted, hasSecrets := spec.redactValue(md); hasSecrets {
		spec.Value = redacted
	}
}

// redactValue returns the redacted value of the key, and whether it held any secret.
func (spec *MetaKeySpec) redactValue(md resource.Metadata) (string, bool) {
	if spec.Value == "" {
		return spec.Value, false
	}

	redactor, sensitive := sensitiveMetaKeys[md.ID()]
	if !sensitive {
		return spec.Value, false
	}

	return redactor(spec.Value)
}

// NewMetaKey initializes a MetaKey resource.
func NewMetaKey(namespace resource.Namespace, id resource.ID) *MetaKey {
	return typed.NewResource[MetaKeySpec, MetaKeyExtension](
		resource.NewMetadata(namespace, MetaKeyType, id, resource.VersionUndefined),
		MetaKeySpec{},
	)
}

// MetaKeyExtension is auxiliary resource data for MetaKey.
type MetaKeyExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MetaKeyExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MetaKeyType,
		Aliases:          []resource.Type{"meta"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Value",
				JSONPath: `{.value}`,
			},
		},
		// a few of the META keys hold secrets, see sensitiveMetaKeys
		Sensitivity: meta.Sensitive,
	}
}

// MetaKeyTagToID converts a tag to ID for MetaKey resource.
func MetaKeyTagToID(t uint8) resource.ID {
	return fmt.Sprintf("0x%02x", t)
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[MetaKeySpec](MetaKeyType, &MetaKey{})
	if err != nil {
		panic(err)
	}

	// the spec is sensitive, but it is safe to be read in the redacted form
	redactable.Register[MetaKeySpec, MetaKeyExtension]()
}
