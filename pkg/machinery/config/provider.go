// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// Encoder provides the interface to encode configuration documents.
type Encoder = config.Encoder

// Container provides the interface to access configuration documents.
//
// Container might contain multiple config documents, supporting encoding/decoding,
// validation, and other operations.
type Container interface {
	Encoder

	// Validate checks configuration and returns warnings and fatal errors (as multierror).
	Validate(validation.RuntimeMode, ...validation.Option) ([]string, error)

	// RedactSecrets returns a copy of the Provider with all secrets replaced with the given string.
	RedactSecrets(string) Encoder

	// RawV1Alpha1 returns internal config representation.
	RawV1Alpha1() *v1alpha1.Config
}

// Provider defines the configuration consumption interface combining access and encoding/decoding.
type Provider interface {
	Config
	Container
}
