// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"crypto/rand"
	"io"

	"github.com/jxskiss/base62"

	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/cluster"
)

// IdentitySpec adapter provides identity generation.
//
//nolint:revive,golint
func IdentitySpec(r *cluster.IdentitySpec) identity {
	return identity{
		IdentitySpec: r,
	}
}

type identity struct {
	*cluster.IdentitySpec
}

// Generate new identity.
func (a identity) Generate() error {
	buf := make([]byte, constants.DefaultNodeIdentitySize)

	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return err
	}

	a.IdentitySpec.NodeID = base62.EncodeToString(buf)

	return nil
}
