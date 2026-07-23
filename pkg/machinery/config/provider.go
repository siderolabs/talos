// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"context"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// Encoder provides the interface to encode configuration documents.
type Encoder = config.Encoder

// Validator provides the interface to validate configuration.
type Validator = config.Validator

// RuntimeValidator provides the interface to validate configuration in the runtime context.
type RuntimeValidator = config.RuntimeValidator

// Container provides the interface to access configuration documents.
//
// Container might contain multiple config documents, supporting encoding/decoding,
// validation, and other operations.
type Container interface {
	Encoder

	// Validate checks configuration and returns warnings and fatal errors (as multierror).
	//
	// Deprecated: use ValidateAsClient instead for client-side validation (outside of Talos).
	Validate(validation.RuntimeMode, ...validation.Option) ([]string, error)

	// RuntimeValidate validates the config in the runtime context.
	//
	// The method returns warnings and fatal errors (as multierror).
	//
	// Deprecated: use ValidateAtRuntime instead for runtime validation (inside Talos).
	RuntimeValidate(context.Context, state.State, validation.RuntimeMode, ...validation.Option) ([]string, error)

	// ValidateAsClient validates the config in the client context (outside of Talos).
	//
	// The method returns warnings and fatal errors (as multierror).
	ValidateAsClient(validation.RuntimeMode, ...validation.Option) ([]string, error)

	// ValidateAtRuntime validates the config in the runtime context (inside Talos).
	//
	// The method returns warnings and fatal errors (as multierror).
	ValidateAtRuntime(context.Context, state.State, validation.RuntimeMode, ...validation.Option) ([]string, error)

	Readonly() bool

	// RawV1Alpha1 returns internal config representation.
	RawV1Alpha1() *v1alpha1.Config

	// Documents returns a list of config documents.
	//
	// Documents should be not be modified.
	Documents() []config.Document

	// Has checks if the container has a config document of the given kind.
	//
	// This method doesn't support legacy v1alpha1 config.
	Has(kind string) bool
}

// Provider defines the configuration consumption interface combining access and encoding/decoding.
type Provider interface {
	Config
	Container

	// Clone returns a copy of the Provider.
	Clone() Provider

	// PatchV1Alpha1 patches the container's v1alpha1.Config while preserving other config documents.
	PatchV1Alpha1(patcher func(*v1alpha1.Config) error) (Provider, error)

	// RedactSecrets returns a copy of the Provider with all secrets replaced with the given string.
	RedactSecrets(string) Provider

	// CompleteForBoot return true if the machine config is enough to proceed with the boot process.
	CompleteForBoot() bool
}
