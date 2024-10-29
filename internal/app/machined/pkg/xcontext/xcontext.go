// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package xcontext provides a small utils for context package
package xcontext

import "context"

// AfterFuncSync is like [context.AfterFunc] but it blocks until the function is executed.
func AfterFuncSync(ctx context.Context, fn func()) func() bool {
	stopChan := make(chan struct{})

	stop := context.AfterFunc(ctx, func() {
		defer close(stopChan)

		fn()
	})

	return func() bool {
		result := stop()
		if !result {
			<-stopChan
		}

		return result
	}
}
