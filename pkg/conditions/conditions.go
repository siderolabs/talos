// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conditions

import (
	"context"
	"fmt"
)

// OK is returned by the String method of the passed Condition.
const OK = "OK"

// Condition is a object which Wait()s for some condition to become true.
//
// Condition can describe itself via String() method.
type Condition interface {
	fmt.Stringer
	Wait(ctx context.Context) error
}
