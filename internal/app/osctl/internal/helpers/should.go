/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package helpers

// Should panics if err != nil
//
// Should is useful when error should never happen in customer environment,
// it can only be development error.
func Should(err error) {
	if err != nil {
		panic(err)
	}
}
