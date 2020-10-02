// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package errors

import "errors"

// ErrNoConfigSource indicates that the platform does not have a configured source for the configuration.
var ErrNoConfigSource = errors.New("no configuration source")
