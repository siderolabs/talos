// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package container implements a wrapper which wraps all configuration documents into a single container.
package container

import (
	"bytes"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/slices"

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

	for _, doc := range documents {
		switch d := doc.(type) {
		case *v1alpha1.Config:
			if container.v1alpha1Config != nil {
				return nil, fmt.Errorf("duplicate v1alpha1.Config")
			}

			container.v1alpha1Config = d
		default:
			// TODO: we should check for some uniqueness of multi-docs (?)
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
		documents:      slices.Map(container.documents, config.Document.Clone),
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

// Persist implements config.Config interface.
func (container *Container) Persist() bool {
	if container.v1alpha1Config == nil {
		return false
	}

	return container.v1alpha1Config.Persist()
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

// SideroLink implements config.Config interface.
func (container *Container) SideroLink() config.SideroLinkConfig {
	for _, doc := range container.documents {
		if c, ok := doc.(config.SideroLinkConfig); ok {
			return c
		}
	}

	return nil
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
			buf.Write([]byte("---\n"))
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

	for _, doc := range container.documents {
		if validatableDoc, ok := doc.(config.Validator); ok {
			docWarnings, docErr := validatableDoc.Validate(mode, opt...)

			warnings = append(warnings, docWarnings...)
			err = multierror.Append(err, docErr)
		}
	}

	return warnings, err
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

// RawV1Alpha1 returns internal config representation.
func (container *Container) RawV1Alpha1() *v1alpha1.Config {
	if container.readonly {
		return container.v1alpha1Config.DeepCopy()
	}

	return container.v1alpha1Config
}
