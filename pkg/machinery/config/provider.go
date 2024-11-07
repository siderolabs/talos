// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
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
	Validator
	RuntimeValidator

	Readonly() bool

	// RawV1Alpha1 returns internal config representation.
	RawV1Alpha1() *v1alpha1.Config

	// Documents returns a list of config documents.
	//
	// Documents should be not be modified.
	Documents() []config.Document
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
