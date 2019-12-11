// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runner

import (
	"context"

	"github.com/talos-systems/talos/internal/test-framework/pkg/checker"
)

// Runner can be used to run a command in the given context.
type Runner interface {
	Run(context.Context, checker.Check) error
	Check(context.Context, checker.Check) error
	Cleanup(context.Context) error
}
