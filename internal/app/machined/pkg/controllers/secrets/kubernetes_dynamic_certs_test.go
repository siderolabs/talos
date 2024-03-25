// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	stdlibx509 "crypto/x509"
	"fmt"
	"net/netip"
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	secretsctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	timeresource "github.com/siderolabs/talos/pkg/machinery/resources/time"
)

func TestKubernetesDynamicCertsSuite(t *testing.T) {
	suite.Run(t, &KubernetesDynamicCertsSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&secretsctrl.KubernetesDynamicCertsController{}))
			},
		},
	})
}

type KubernetesDynamicCertsSuite struct {
	ctest.DefaultSuite
}

func (suite *KubernetesDynamicCertsSuite) TestReconcile() {
	rootSecrets := secrets.NewKubernetesRoot(secrets.KubernetesRootID)

	k8sCA, err := x509.NewSelfSignedCertificateAuthority(
		x509.Organization("kubernetes"),
		x509.ECDSA(true),
	)
	suite.Require().NoError(err)

	aggregatorCA, err := x509.NewSelfSignedCertificateAuthority(
		x509.Organization("kubernetes"),
		x509.ECDSA(true),
	)
	suite.Require().NoError(err)

	serviceAccount, err := x509.NewECDSAKey()
	suite.Require().NoError(err)

	rootSecrets.TypedSpec().Name = "cluster1"
	rootSecrets.TypedSpec().Endpoint, err = url.Parse("https://some.url:6443/")
	suite.Require().NoError(err)
	rootSecrets.TypedSpec().LocalEndpoint, err = url.Parse("https://localhost:6443/")
	suite.Require().NoError(err)

	rootSecrets.TypedSpec().IssuingCA = &x509.PEMEncodedCertificateAndKey{
		Crt: k8sCA.CrtPEM,
		Key: k8sCA.KeyPEM,
	}
	rootSecrets.TypedSpec().AggregatorCA = &x509.PEMEncodedCertificateAndKey{
		Crt: aggregatorCA.CrtPEM,
		Key: aggregatorCA.KeyPEM,
	}
	rootSecrets.TypedSpec().ServiceAccount = &x509.PEMEncodedKey{
		Key: serviceAccount.KeyPEM,
	}
	rootSecrets.TypedSpec().CertSANs = []string{"example.com"}
	rootSecrets.TypedSpec().APIServerIPs = []netip.Addr{netip.MustParseAddr("10.4.3.2"), netip.MustParseAddr("10.2.1.3")}
	rootSecrets.TypedSpec().DNSDomain = "cluster.remote"
	suite.Require().NoError(suite.State().Create(suite.Ctx(), rootSecrets))

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeControlPlane)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), machineType))

	networkStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	networkStatus.TypedSpec().AddressReady = true
	networkStatus.TypedSpec().HostnameReady = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), networkStatus))

	certSANs := secrets.NewCertSAN(secrets.NamespaceName, secrets.CertSANKubernetesID)
	certSANs.TypedSpec().Append(
		"example.com",
		"foo",
		"foo.example.com",
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc.cluster.remote",
		"localhost",
		"some.url",
		"10.2.1.3",
		"10.4.3.2",
		"172.16.0.1",
	)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), certSANs))

	timeSync := timeresource.NewStatus()
	*timeSync.TypedSpec() = timeresource.StatusSpec{
		Synced: true,
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), timeSync))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.KubernetesDynamicCertsID},
		func(certs *secrets.KubernetesDynamicCerts, assertion *assert.Assertions) {
			kubernetesCerts := certs.TypedSpec()

			apiCert, err := kubernetesCerts.APIServer.GetCert()
			assertion.NoError(err)

			if err != nil {
				return
			}

			assertion.Equal(
				[]string{
					"example.com",
					"foo",
					"foo.example.com",
					"kubernetes",
					"kubernetes.default",
					"kubernetes.default.svc",
					"kubernetes.default.svc.cluster.remote",
					"localhost",
					"some.url",
				}, apiCert.DNSNames,
			)
			assertion.Equal("[10.2.1.3 10.4.3.2 172.16.0.1]", fmt.Sprintf("%v", apiCert.IPAddresses))

			assertion.Equal("kube-apiserver", apiCert.Subject.CommonName)
			assertion.Equal([]string{"kube-master"}, apiCert.Subject.Organization)

			assertion.Equal(
				stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment,
				apiCert.KeyUsage,
			)
			assertion.Equal([]stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageServerAuth}, apiCert.ExtKeyUsage)

			clientCert, err := kubernetesCerts.APIServerKubeletClient.GetCert()
			assertion.NoError(err)

			if err != nil {
				return
			}

			assertion.Empty(clientCert.DNSNames)
			assertion.Empty(clientCert.IPAddresses)

			assertion.Equal(
				constants.KubernetesAPIServerKubeletClientCommonName,
				clientCert.Subject.CommonName,
			)
			assertion.Equal(
				[]string{constants.KubernetesAdminCertOrganization},
				clientCert.Subject.Organization,
			)

			assertion.Equal(
				stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment,
				clientCert.KeyUsage,
			)
			assertion.Equal([]stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageClientAuth}, clientCert.ExtKeyUsage)

			frontProxyCert, err := kubernetesCerts.FrontProxy.GetCert()
			assertion.NoError(err)

			if err != nil {
				return
			}

			assertion.Empty(frontProxyCert.DNSNames)
			assertion.Empty(frontProxyCert.IPAddresses)

			assertion.Equal("front-proxy-client", frontProxyCert.Subject.CommonName)
			assertion.Empty(frontProxyCert.Subject.Organization)

			assertion.Equal(
				stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment,
				frontProxyCert.KeyUsage,
			)
			assertion.Equal(
				[]stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageClientAuth},
				frontProxyCert.ExtKeyUsage,
			)
		})
}
