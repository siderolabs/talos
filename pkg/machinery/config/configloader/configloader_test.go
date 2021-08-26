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
	for _, tt := range []struct {
		source      []byte
		expectedErr string
	}{
		{
			source:      []byte(":   \xea"),
			expectedErr: "recovered: internal error: attempted to parse unknown event (please report): none",
		},
	} {
		_, err := newConfig(tt.source)

		if tt.expectedErr == "" {
			suite.Require().NoError(err)
		} else {
			suite.Require().EqualError(err, tt.expectedErr)
		}
	}
}
