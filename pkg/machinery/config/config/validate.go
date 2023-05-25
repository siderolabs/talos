// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import "github.com/siderolabs/talos/pkg/machinery/config/validation"

// Validator is the interface to validate configuration.
//
// Validator might be implemented by a Container and a single Document.
type Validator interface {
	// Validate checks configuration and returns warnings and fatal errors (as multierror).
	Validate(validation.RuntimeMode, ...validation.Option) ([]string, error)
}
