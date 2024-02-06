// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package container implements a wrapper which wraps all configuration documents into a single container.
package container

import (
	"bytes"
	"errors"
	"fmt"
	"slices"

	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/xslices"

	coreconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// Container wraps all configuration documents into a single container.
type Container struct {
	v1alpha1Config *v1alpha1.Config
	documents      []config.Document
	bytes          []byte
	readonly       bool
}

var _ coreconfig.Provider = &Container{}

// New creates a container out of the list of documents.
func New(documents ...config.Document) (*Container, error) {
	container := &Container{
		documents: make([]config.Document, 0, len(documents)),
	}

	seenDocuments := make(map[string]struct{})

	for _, doc := range documents {
		switch d := doc.(type) {
		case *v1alpha1.Config:
			if container.v1alpha1Config != nil {
				return nil, errors.New("duplicate v1alpha1.Config")
			}

			container.v1alpha1Config = d
		default:
			documentID := d.Kind() + "/"

			if named, ok := d.(config.NamedDocument); ok {
				documentID += named.Name()
			}

			if _, alreadySeen := seenDocuments[documentID]; alreadySeen {
				return nil, fmt.Errorf("duplicate document: %s", documentID)
			}

			seenDocuments[documentID] = struct{}{}

			container.documents = append(container.documents, d)
		}
	}

	return container, nil
}

// NewReadonly creates a read-only container which preserves byte representation of contents.
func NewReadonly(bytes []byte, documents ...config.Document) (*Container, error) {
	c, err := New(documents...)
	if err != nil {
		return nil, err
	}

	c.bytes = bytes
	c.readonly = true

	return c, nil
}

// NewV1Alpha1 creates a container with (only) v1alpha1.Config document.
func NewV1Alpha1(config *v1alpha1.Config) *Container {
	return &Container{
		v1alpha1Config: config,
	}
}

// Clone the container.
//
// Cloned container is not readonly.
func (container *Container) Clone() coreconfig.Provider {
	return &Container{
		v1alpha1Config: container.v1alpha1Config.DeepCopy(),
		documents:      xslices.Map(container.documents, config.Document.Clone),
	}
}

// Readonly implements config.Container interface.
func (container *Container) Readonly() bool {
	return container.readonly
}

// Debug implements config.Config interface.
func (container *Container) Debug() bool {
	if container.v1alpha1Config == nil {
		return false
	}

	return container.v1alpha1Config.Debug()
}

// Machine implements config.Config interface.
func (container *Container) Machine() config.MachineConfig {
	if container.v1alpha1Config == nil {
		return nil
	}

	return container.v1alpha1Config.Machine()
}

// Cluster implements config.Config interface.
func (container *Container) Cluster() config.ClusterConfig {
	if container.v1alpha1Config == nil {
		return nil
	}

	return container.v1alpha1Config.Cluster()
}

func findMatchingDocs[T any](documents []config.Document) []T {
	var result []T

	for _, doc := range documents {
		if c, ok := doc.(T); ok {
			result = append(result, c)
		}
	}

	return result
}

// SideroLink implements config.Config interface.
func (container *Container) SideroLink() config.SideroLinkConfig {
	matching := findMatchingDocs[config.SideroLinkConfig](container.documents)
	if len(matching) == 0 {
		return nil
	}

	return matching[0]
}

// ExtensionServiceConfigs implements config.Config interface.
func (container *Container) ExtensionServiceConfigs() []config.ExtensionServiceConfig {
	return findMatchingDocs[config.ExtensionServiceConfig](container.documents)
}

// Runtime implements config.Config interface.
func (container *Container) Runtime() config.RuntimeConfig {
	return config.WrapRuntimeConfigList(findMatchingDocs[config.RuntimeConfig](container.documents)...)
}

// NetworkRules implements config.Config interface.
func (container *Container) NetworkRules() config.NetworkRuleConfig {
	return config.WrapNetworkRuleConfigList(findMatchingDocs[config.NetworkRuleConfigSignal](container.documents)...)
}

// Bytes returns source YAML representation (if available) or does default encoding.
func (container *Container) Bytes() ([]byte, error) {
	if !container.readonly {
		return container.EncodeBytes()
	}

	if container.bytes == nil {
		panic("container.Bytes() called on a readonly container without bytes")
	}

	return container.bytes, nil
}

// EncodeString configuration to YAML using the provided options.
func (container *Container) EncodeString(encoderOptions ...encoder.Option) (string, error) {
	b, err := container.EncodeBytes(encoderOptions...)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// EncodeBytes configuration to YAML using the provided options.
func (container *Container) EncodeBytes(encoderOptions ...encoder.Option) ([]byte, error) {
	var buf bytes.Buffer

	if container.v1alpha1Config != nil {
		b, err := encoder.NewEncoder(container.v1alpha1Config, encoderOptions...).Encode()
		if err != nil {
			return nil, err
		}

		buf.Write(b)
	}

	for _, doc := range container.documents {
		if buf.Len() > 0 {
			buf.WriteString("---\n")
		}

		b, err := encoder.NewEncoder(doc, encoderOptions...).Encode()
		if err != nil {
			return nil, err
		}

		buf.Write(b)
	}

	return buf.Bytes(), nil
}

// Validate checks configuration and returns warnings and fatal errors (as multierror).
func (container *Container) Validate(mode validation.RuntimeMode, opt ...validation.Option) ([]string, error) {
	var (
		warnings []string
		err      error
	)

	if container.v1alpha1Config != nil {
		warnings, err = container.v1alpha1Config.Validate(mode, opt...)
	}

	var multiErr *multierror.Error

	if err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	for _, doc := range container.documents {
		if validatableDoc, ok := doc.(config.Validator); ok {
			docWarnings, docErr := validatableDoc.Validate(mode, opt...)

			warnings = append(warnings, docWarnings...)
			multiErr = multierror.Append(multiErr, docErr)
		}
	}

	return warnings, multiErr.ErrorOrNil()
}

// RedactSecrets returns a copy of the Provider with all secrets replaced with the given string.
func (container *Container) RedactSecrets(replacement string) coreconfig.Provider {
	clone := container.Clone().(*Container) //nolint:forcetypeassert,errcheck

	if clone.v1alpha1Config != nil {
		clone.v1alpha1Config.Redact(replacement)
	}

	for _, doc := range clone.documents {
		if secretDoc, ok := doc.(config.SecretDocument); ok {
			secretDoc.Redact(replacement)
		}
	}

	return clone
}

// RawV1Alpha1 returns internal config representation for v1alpha1.Config.
func (container *Container) RawV1Alpha1() *v1alpha1.Config {
	if container.readonly {
		return container.v1alpha1Config.DeepCopy()
	}

	return container.v1alpha1Config
}

// Documents returns all documents in the container.
//
// Documents should not be modified.
func (container *Container) Documents() []config.Document {
	docs := slices.Clone(container.documents)

	if container.v1alpha1Config != nil {
		docs = append([]config.Document{container.v1alpha1Config}, docs...)
	}

	return docs
}

// CompleteForBoot return true if the machine config is enough to proceed with the boot process.
func (container *Container) CompleteForBoot() bool {
	// for now, v1alpha1 config is required
	return container.v1alpha1Config != nil
}
