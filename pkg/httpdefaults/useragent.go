// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package httpdefaults

import "github.com/siderolabs/talos/pkg/machinery/version"

// UserAgent is the default User-Agent header value for HTTP requests made by Talos.
func UserAgent() string {
	return version.Name + "/" + version.Tag
}
