// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !linux

package mgmt

import (
	"context"
	"errors"
)

func runLLDPAdvertiser(context.Context, string) error {
	return errors.New("the LLDP advertiser requires a Linux QEMU host; use the remote provisioner")
}
