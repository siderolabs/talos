/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package kernel_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/pkg/kernel"
)

type KernelSuite struct {
	suite.Suite
}

func (suite *KernelSuite) TestParseKernelBootParameters() {
	for _, t := range []struct {
		params   []byte
		expected map[string]string
	}{
		{[]byte(""), map[string]string{}},
		{[]byte("boot=xyz root=/dev/abc nogui"), map[string]string{"boot": "xyz", "root": "/dev/abc", "nogui": ""}},
		{[]byte(" root=/dev/abc=1  nogui  \n"), map[string]string{"root": "/dev/abc=1", "nogui": ""}},
		{[]byte("root=/dev/sda root=/dev/sdb"), map[string]string{"root": "/dev/sdb"}},
	} {
		suite.Assert().Equal(t.expected, kernel.ParseKernelBootParameters(t.params))
	}

}

func TestKernelSuite(t *testing.T) {
	suite.Run(t, new(KernelSuite))
}
