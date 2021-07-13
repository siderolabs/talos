// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package wglan

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type NetworkdSuite struct {
	suite.Suite
}

func TestNetworkdSuite(t *testing.T) {
	suite.Run(t, new(NetworkdSuite))
}
