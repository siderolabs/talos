// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ntp

import (
	"crypto/tls"

	"github.com/beevik/nts"

	"github.com/siderolabs/talos/pkg/httpdefaults"
)

// DefaultNTSNewSession creates a real NTS session using beevik/nts.
// This is the default NTSNewSessionFunc used in production.
func DefaultNTSNewSession(address string) (NTSSession, error) {
	return nts.NewSessionWithOptions(
		address,
		&nts.SessionOptions{
			TLSConfig: &tls.Config{
				RootCAs: httpdefaults.RootCAs(),
			},
		},
	)
}
