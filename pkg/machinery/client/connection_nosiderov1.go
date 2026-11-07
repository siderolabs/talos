// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build no_siderov1_auth

package client

import (
	"errors"

	"google.golang.org/grpc"

	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
)

// sideroV1DialOptions fails: SideroV1 auth support is excluded from this build by the no_siderov1_auth build tag.
//
// The build tag is used for binaries which never connect with SideroV1 auth (e.g. machined)
// to avoid linking in the request signing code and its dependencies.
func (c *Client) sideroV1DialOptions(*clientconfig.SideroV1) ([]grpc.DialOption, error) {
	return nil, errors.New("SideroV1 auth is not supported in this build")
}
