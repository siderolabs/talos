// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conditions

import "context"

type condition struct{}

func (condition) Wait(ctx context.Context) error {
	return nil
}

func (condition) String() string {
	return "nothing"
}

// None is a service condition that has no conditions.
func None() Condition {
	return condition{}
}
