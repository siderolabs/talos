/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"strings"

	ud "github.com/talos-systems/talos/pkg/userdata"
)

// UserData provides an abstraction to call the appropriate method to
// load user data
// TODO: Merge this in to internal/pkg/userdata
func UserData(location string) (userData *ud.UserData, err error) {
	if strings.HasPrefix(location, "http") {
		userData, err = ud.Download(location, nil)
	} else {
		userData, err = ud.Open(location)
	}
	return userData, err
}
