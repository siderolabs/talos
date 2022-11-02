// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"crypto/tls"

	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
)

func ReduceURLsToAddresses(endpoints []string) []string {
	return reduceURLsToAddresses(endpoints)
}

func BuildTLSConfig(configContext *clientconfig.Context) (*tls.Config, error) {
	return buildTLSConfig(configContext)
}
