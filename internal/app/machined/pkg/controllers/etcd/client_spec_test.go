// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	etcdctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

func TestClientSpecSuite(t *testing.T) {
	t.Parallel()

	s := &ClientSpecSuite{}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: time.Minute,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&etcdctrl.ClientSpecController{}))
		},
	}

	suite.Run(t, s)
}

type ClientSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *ClientSpecSuite) TestReconcile() {
	rootSecrets := secrets.NewEtcdRoot(secrets.EtcdRootID)
	rootSecrets.TypedSpec().AcceptedCAs = []*x509.PEMEncodedCertificate{{Crt: []byte("ca")}}
	suite.Create(rootSecrets)

	// no client certificate yet
	ctest.AssertNoResource[*etcd.ClientSpec](suite, etcd.ClientSpecID)

	etcdSecrets := secrets.NewEtcd()
	etcdSecrets.TypedSpec().EtcdAdmin = &x509.PEMEncodedCertificateAndKey{Crt: []byte("crt"), Key: []byte("key")}
	suite.Create(etcdSecrets)

	// the control plane machines are not discovered yet
	ctest.AssertResource(suite, etcd.ClientSpecID, func(spec *etcd.ClientSpec, asrt *assert.Assertions) {
		asrt.Equal([]*x509.PEMEncodedCertificate{{Crt: []byte("ca")}}, spec.TypedSpec().AcceptedCAs)
		asrt.Equal(&x509.PEMEncodedCertificateAndKey{Crt: []byte("crt"), Key: []byte("key")}, spec.TypedSpec().Certificate)
		asrt.Equal([]string{"localhost:2379"}, spec.TypedSpec().LocalEndpoints)
		asrt.Empty(spec.TypedSpec().ClusterEndpoints)
	})

	endpoints := k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, "discovery")
	endpoints.TypedSpec().Addresses = []netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("fd00::2")}
	suite.Create(endpoints)

	ctest.AssertResource(suite, etcd.ClientSpecID, func(spec *etcd.ClientSpec, asrt *assert.Assertions) {
		asrt.Equal([]string{"10.0.0.1:2379", "[fd00::2]:2379"}, spec.TypedSpec().ClusterEndpoints)
	})

	// the credentials are gone
	suite.Destroy(etcdSecrets)

	ctest.AssertNoResource[*etcd.ClientSpec](suite, etcd.ClientSpecID)
}
