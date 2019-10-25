// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ntp

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type NtpSuite struct {
	suite.Suite
}

func TestNtpSuite(t *testing.T) {
	suite.Run(t, new(NtpSuite))
}

func (suite *NtpSuite) TestQuery() {
	testServer := "time.cloudflare.com"
	// Create ntp client
	n, err := NewNTPClient(WithServer(testServer))
	suite.Assert().NoError(err)

	_, err = n.Query()
	suite.Assert().NoError(err)
}
