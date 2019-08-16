/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type translatorSuite struct {
	suite.Suite
}

func TestTranslatorSuite(t *testing.T) {
	suite.Run(t, new(translatorSuite))
}

func (suite *translatorSuite) TestTranslation() {
	ud, err := TranslateV1(testV1Config)
	suite.Require().NoError(err)
	suite.Assert().Equal(string(ud.Version), "v1")
	err = ud.Validate()
	suite.Require().NoError(err)
}
