// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package configloader

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// docgen: nodoc
type Suite struct {
	suite.Suite
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupSuite() {}

func (suite *Suite) TestNew() {
	for _, t := range []struct {
		source      []byte
		errExpected bool
	}{} {
		_, err := newConfig(t.source)

		if t.errExpected {
			suite.Require().Error(err)
		} else {
			suite.Require().NoError(err)
		}
	}
}
