/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package translate

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
	tv1, err := NewTranslator("v1", testV1Config)
	suite.Require().NoError(err)

	ud, err := tv1.Translate()
	suite.Require().NoError(err)

	suite.Assert().Equal(string(ud.Version), "v1")
	err = ud.Validate()
	suite.Require().NoError(err)
}

// nolint: lll
const testV1Config = `version: v1
machine:
  type: init
  token: 57dn7x.k5jc6dum97cotlqb
  ca:
    crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=
    key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCi0tLS0tRU5EIEVDIFBSSVZBVEUgS0VZLS0tLS0=
  kubelet: {}
  network: {}
  install: {}
cluster:
  controlPlane:
    ips:
    - 10.254.0.10
  clusterName: spencer-test
  network:
    dnsDomain: cluster.local
    podSubnets:
    - 10.244.0.0/16
    serviceSubnets:
    - 10.96.0.0/12
  token: 4iysc6.t3bsjbrd74v91wpv
  initToken: 22c11be4-c413-11e9-b8e8-309c23e4bd47
  ca:
    crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=
    key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCi0tLS0tRU5EIEVDIFBSSVZBVEUgS0VZLS0tLS0=
  apiServer: {}
  controllerManager: {}
  scheduler: {}
  etcd: {}
 `
