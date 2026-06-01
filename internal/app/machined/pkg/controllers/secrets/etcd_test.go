// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	secretsctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	timeres "github.com/siderolabs/talos/pkg/machinery/resources/time"
)

func TestEtcdSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &EtcdSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&secretsctrl.EtcdController{}))
			},
		},
	})
}

type EtcdSuite struct {
	ctest.DefaultSuite
}

func (suite *EtcdSuite) TestReconcile() {
	rootSecrets := secrets.NewEtcdRoot(secrets.EtcdRootID)

	etcdCA, err := x509.NewSelfSignedCertificateAuthority(
		x509.Organization("talos"),
		x509.ECDSA(true),
	)
	suite.Require().NoError(err)

	rootSecrets.TypedSpec().EtcdCA = &x509.PEMEncodedCertificateAndKey{
		Crt: etcdCA.CrtPEM,
		Key: etcdCA.KeyPEM,
	}
	suite.Create(rootSecrets)

	networkStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	networkStatus.TypedSpec().AddressReady = true
	networkStatus.TypedSpec().HostnameReady = true
	suite.Create(networkStatus)

	hostnameStatus := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostnameStatus.TypedSpec().Hostname = "host"
	hostnameStatus.TypedSpec().Domainname = "domain"
	suite.Create(hostnameStatus)

	nodeAddresses := network.NewNodeAddress(network.NamespaceName, network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, k8s.NodeAddressFilterNoK8s))
	nodeAddresses.TypedSpec().Addresses = []netip.Prefix{
		netip.MustParsePrefix("10.3.4.5/24"),
		netip.MustParsePrefix("2001:db8::1eaf/64"),
	}
	suite.Create(nodeAddresses)

	timeSync := timeres.NewStatus()
	timeSync.TypedSpec().Synced = true
	suite.Create(timeSync)

	ctest.AssertResource(suite, secrets.EtcdID, func(certs *secrets.Etcd, asrt *assert.Assertions) {
		etcdCerts := certs.TypedSpec()

		serverCert, err := etcdCerts.Etcd.GetCert()
		if !asrt.NoError(err) {
			return
		}

		asrt.Equal([]string{"host", "host.domain", "localhost"}, serverCert.DNSNames)
		asrt.Equal("[10.3.4.5 2001:db8::1eaf 127.0.0.1 ::1]", fmt.Sprintf("%v", serverCert.IPAddresses))

		asrt.Equal("host", serverCert.Subject.CommonName)

		peerCert, err := etcdCerts.EtcdPeer.GetCert()
		if !asrt.NoError(err) {
			return
		}

		asrt.Equal([]string{"host", "host.domain"}, peerCert.DNSNames)
		asrt.Equal("[10.3.4.5 2001:db8::1eaf]", fmt.Sprintf("%v", peerCert.IPAddresses))

		asrt.Equal("host", peerCert.Subject.CommonName)

		adminCert, err := etcdCerts.EtcdAdmin.GetCert()
		if !asrt.NoError(err) {
			return
		}

		asrt.Empty(adminCert.DNSNames)
		asrt.Empty(adminCert.IPAddresses)

		asrt.Equal("talos", adminCert.Subject.CommonName)

		kubeAPICert, err := etcdCerts.EtcdAPIServer.GetCert()
		if !asrt.NoError(err) {
			return
		}

		asrt.Empty(kubeAPICert.DNSNames)
		asrt.Empty(kubeAPICert.IPAddresses)

		asrt.Equal("kube-apiserver", kubeAPICert.Subject.CommonName)
	})

	// update node addresses, certs should be updated
	nodeAddresses.TypedSpec().Addresses = []netip.Prefix{
		netip.MustParsePrefix("10.3.4.5/24"),
	}
	suite.Update(nodeAddresses)

	ctest.AssertResource(suite, secrets.EtcdID, func(certs *secrets.Etcd, asrt *assert.Assertions) {
		etcdCerts := certs.TypedSpec()

		serverCert, err := etcdCerts.Etcd.GetCert()
		if !asrt.NoError(err) {
			return
		}

		asrt.Equal([]string{"host", "host.domain", "localhost"}, serverCert.DNSNames)
		asrt.Equal("[10.3.4.5 127.0.0.1 ::1]", fmt.Sprintf("%v", serverCert.IPAddresses))

		asrt.Equal("host", serverCert.Subject.CommonName)

		peerCert, err := etcdCerts.EtcdPeer.GetCert()
		if !asrt.NoError(err) {
			return
		}

		asrt.Equal([]string{"host", "host.domain"}, peerCert.DNSNames)
		asrt.Equal("[10.3.4.5]", fmt.Sprintf("%v", peerCert.IPAddresses))

		asrt.Equal("host", peerCert.Subject.CommonName)
	})
}
