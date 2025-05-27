// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package container implements a wrapper which wraps all configuration documents into a single container.
package container

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/state"
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
			if _, ok := d.(selector); !ok {
				documentID := d.Kind() + "/"

				if named, ok := d.(config.NamedDocument); ok {
					documentID += named.Name()
				}

				if _, alreadySeen := seenDocuments[documentID]; alreadySeen {
					return nil, fmt.Errorf("duplicate document: %s", documentID)
				}

				seenDocuments[documentID] = struct{}{}
			}

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
func (container *Container) Clone() coreconfig.Provider { return container.clone() }

func (container *Container) clone() *Container {
	return &Container{
		v1alpha1Config: container.v1alpha1Config.DeepCopy(),
		documents:      xslices.Map(container.documents, config.Document.Clone),
	}
}

// PatchV1Alpha1 patches the container's v1alpha1.Config while preserving other config documents.
func (container *Container) PatchV1Alpha1(patcher func(*v1alpha1.Config) error) (coreconfig.Provider, error) {
	cfg := container.RawV1Alpha1()
	if cfg == nil {
		return nil, fmt.Errorf("v1alpha1.Config is not present in the container")
	}

	cfg = cfg.DeepCopy()

	if err := patcher(cfg); err != nil {
		return nil, err
	}

	otherDocs := xslices.Filter(container.Documents(), func(doc config.Document) bool {
		_, ok := doc.(*v1alpha1.Config)

		return !ok
	})

	return New(slices.Insert(otherDocs, 0, config.Document(cfg))...)
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

// TrustedRoots implements config.Config interface.
func (container *Container) TrustedRoots() config.TrustedRootsConfig {
	return config.WrapTrustedRootsConfig(findMatchingDocs[config.TrustedRootsConfig](container.documents)...)
}

// Volumes implements config.Config interface.
func (container *Container) Volumes() config.VolumesConfig {
	return config.WrapVolumesConfigList(findMatchingDocs[config.VolumeConfig](container.documents)...)
}

// KubespanConfig implements config.Config interface.
func (container *Container) KubespanConfig() config.KubespanConfig {
	return config.WrapKubespanConfig(findMatchingDocs[config.KubespanConfig](container.documents)...)
}

// PCIDriverRebindConfig implements config.Config interface.
func (container *Container) PCIDriverRebindConfig() config.PCIDriverRebindConfig {
	return config.WrapPCIDriverRebindConfig(findMatchingDocs[config.PCIDriverRebindConfig](container.documents)...)
}

// EthernetConfigs implements config.Config interface.
func (container *Container) EthernetConfigs() []config.EthernetConfig {
	return findMatchingDocs[config.EthernetConfig](container.documents)
}

// UserVolumeConfigs implements config.Config interface.
func (container *Container) UserVolumeConfigs() []config.UserVolumeConfig {
	return findMatchingDocs[config.UserVolumeConfig](container.documents)
}

// SwapVolumeConfigs implements config.Config interface.
func (container *Container) SwapVolumeConfigs() []config.SwapVolumeConfig {
	return findMatchingDocs[config.SwapVolumeConfig](container.documents)
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
	var buf strings.Builder

	err := container.encodeToBuf(&buf, encoderOptions...)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// EncodeBytes configuration to YAML using the provided options.
func (container *Container) EncodeBytes(encoderOptions ...encoder.Option) ([]byte, error) {
	var buf bytes.Buffer

	err := container.encodeToBuf(&buf, encoderOptions...)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type buffer interface {
	Len() int
	Write(p []byte) (int, error)
	WriteString(s string) (int, error)
}

func (container *Container) encodeToBuf(buf buffer, encoderOptions ...encoder.Option) error {
	if container.v1alpha1Config != nil {
		b, err := encoder.NewEncoder(container.v1alpha1Config, encoderOptions...).Encode()
		if err != nil {
			return err
		}

		buf.Write(b) //nolint:errcheck
	}

	for _, doc := range container.documents {
		if buf.Len() > 0 {
			buf.WriteString("---\n") //nolint:errcheck
		}

		b, err := encoder.NewEncoder(doc, encoderOptions...).Encode()
		if err != nil {
			return err
		}

		buf.Write(b) //nolint:errcheck
	}

	return nil
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

// RuntimeValidate validates the config in the runtime context.
func (container *Container) RuntimeValidate(ctx context.Context, st state.State, mode validation.RuntimeMode, opt ...validation.Option) ([]string, error) {
	var (
		warnings []string
		err      error
	)

	if container.v1alpha1Config != nil {
		warnings, err = container.v1alpha1Config.RuntimeValidate(ctx, st, mode, opt...)
	}

	var multiErr *multierror.Error

	if err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	for _, doc := range container.documents {
		if validatableDoc, ok := doc.(config.RuntimeValidator); ok {
			docWarnings, docErr := validatableDoc.RuntimeValidate(ctx, st, mode, opt...)

			warnings = append(warnings, docWarnings...)
			multiErr = multierror.Append(multiErr, docErr)
		}
	}

	return warnings, multiErr.ErrorOrNil()
}

// RedactSecrets returns a copy of the Provider with all secrets replaced with the given string.
func (container *Container) RedactSecrets(replacement string) coreconfig.Provider {
	clone := container.clone()

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
	result := make([]config.Document, 0, len(container.documents)+1)

	// first we take deletes for v1alpha1
	for _, doc := range container.documents {
		if _, ok := doc.(selector); ok && doc.Kind() == v1alpha1.Version {
			result = append(result, doc)
		}
	}

	// then we take the v1alpha1 config
	if container.v1alpha1Config != nil {
		result = append(result, container.v1alpha1Config)
	}

	// then we take the rest
	for _, doc := range container.documents {
		if _, ok := doc.(selector); ok && doc.Kind() == v1alpha1.Version {
			continue
		}

		result = append(result, doc)
	}

	return result
}

type selector interface{ ApplyTo(config.Document) error }

// CompleteForBoot return true if the machine config is enough to proceed with the boot process.
func (container *Container) CompleteForBoot() bool {
	// for now, v1alpha1 config is required
	return container.v1alpha1Config != nil
}
