// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/pkg/config/types/v1alpha1"
)

type Suite struct {
	suite.Suite
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupSuite() {}

func (suite *Suite) TestNew() {
	for _, t := range []struct {
		content     Content
		errExpected bool
	}{
		{Content{Version: v1alpha1.Version}, false},
		{Content{Version: ""}, true},
	} {
		_, err := newConfig(t.content)

		if t.errExpected {
			suite.Require().Error(err)
		} else {
			suite.Require().NoError(err)
		}
	}
}
