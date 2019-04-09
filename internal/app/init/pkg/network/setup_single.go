/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package network

import (
	"log"

	"github.com/talos-systems/talos/pkg/userdata"
)

func setupSingleLink(netconf userdata.Device) (err error) {
	log.Printf("bringing up single link interface %s", netconf.Interface)
	return ifup(netconf.Interface)
}
