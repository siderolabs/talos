// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package extensions contains Talos extensions specific API.
package extensions

// AllowedPaths lists paths allowed in the extension images.
var AllowedPaths = []string{
	"/etc/cri/conf.d",
	"/lib/firmware",
	"/usr/local",
}
