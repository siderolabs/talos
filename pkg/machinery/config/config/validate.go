// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"context"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// Validator is the interface to validate configuration.
//
// Validator might be implemented by a Container and a single Document.
type Validator interface {
	// Validate checks configuration and returns warnings and fatal errors (as multierror).
	Validate(validation.RuntimeMode, ...validation.Option) ([]string, error)
}

// RuntimeValidator is the interface to validate configuration in the runtime context.
//
// This interface is used by Talos itself to validate configuration on the machine (vs. the Validator interface).
//
// The errors reported by Validator & RuntimeValidator are different.
type RuntimeValidator interface {
	// RuntimeValidate validates the config in the runtime context.
	//
	// The method returns warnings and fatal errors (as multierror).
	RuntimeValidate(context.Context, state.State, validation.RuntimeMode, ...validation.Option) ([]string, error)
}
