// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package azure_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/azure"
)

type ConfigSuite struct {
	suite.Suite
}

func (suite *ConfigSuite) TestNetworkPublicIPs() {
	cfg := []byte(`
[
  {
    "ipv4": {
      "ipAddress": [
        {
          "privateIpAddress": "172.18.1.10",
          "publicIpAddress": "1.2.3.4"
        }
      ],
      "subnet": [
        {
          "address": "172.18.1.0",
          "prefix": "24"
        }
      ]
    },
    "ipv6": {
      "ipAddress": [
        {
            "privateIpAddress": "fd00::10",
            "publicIpAddress": ""
        }
       ]
    },
    "macAddress": "000D3AD631EE"
  }
]
`)

	a := &azure.Azure{}

	publicIPs := []net.IP{net.ParseIP("1.2.3.4")}

	result, err := a.GetPublicIPs(cfg)
	suite.Require().NoError(err)
	suite.Assert().Equal(publicIPs, result)
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
