// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

func WrapOnErr(fn func() error, msg string) func() error {
	return wrapOnErr(fn, msg)
}

func LogError(clientClose func() error) {
	logError(clientClose)
}
