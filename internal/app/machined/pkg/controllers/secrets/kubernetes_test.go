// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package secrets_test

import (
	stdlibx509 "crypto/x509"
	"fmt"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/ctest"
	secretsctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	"github.com/talos-systems/talos/pkg/machinery/resources/secrets"
	timeresource "github.com/talos-systems/talos/pkg/machinery/resources/time"
)

func TestKubernetesSuite(t *testing.T) {
	suite.Run(t, &KubernetesSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&secretsctrl.KubernetesController{}))
			},
		},
	})
}

type KubernetesSuite struct {
	ctest.DefaultSuite
}

func (suite *KubernetesSuite) TestReconcile() {
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

	rootSecrets.TypedSpec().CA = &x509.PEMEncodedCertificateAndKey{
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
	rootSecrets.TypedSpec().APIServerIPs = []net.IP{net.ParseIP("10.4.3.2"), net.ParseIP("10.2.1.3")}
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

	suite.AssertWithin(10*time.Second, 100*time.Millisecond, func() error {
		certs, err := ctest.Get[*secrets.Kubernetes](
			suite,
			resource.NewMetadata(
				secrets.NamespaceName,
				secrets.KubernetesType,
				secrets.KubernetesID,
				resource.VersionUndefined,
			),
		)
		if err != nil {
			if state.IsNotFoundError(err) {
				return retry.ExpectedError(err)
			}

			return err
		}

		kubernetesCerts := certs.TypedSpec()

		apiCert, err := kubernetesCerts.APIServer.GetCert()
		suite.Require().NoError(err)

		suite.Assert().Equal(
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
		suite.Assert().Equal("[10.2.1.3 10.4.3.2 172.16.0.1]", fmt.Sprintf("%v", apiCert.IPAddresses))

		suite.Assert().Equal("kube-apiserver", apiCert.Subject.CommonName)
		suite.Assert().Equal([]string{"kube-master"}, apiCert.Subject.Organization)

		suite.Assert().Equal(
			stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment,
			apiCert.KeyUsage,
		)
		suite.Assert().Equal([]stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageServerAuth}, apiCert.ExtKeyUsage)

		clientCert, err := kubernetesCerts.APIServerKubeletClient.GetCert()
		suite.Require().NoError(err)

		suite.Assert().Empty(clientCert.DNSNames)
		suite.Assert().Empty(clientCert.IPAddresses)

		suite.Assert().Equal(
			constants.KubernetesAPIServerKubeletClientCommonName,
			clientCert.Subject.CommonName,
		)
		suite.Assert().Equal(
			[]string{constants.KubernetesAdminCertOrganization},
			clientCert.Subject.Organization,
		)

		suite.Assert().Equal(
			stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment,
			clientCert.KeyUsage,
		)
		suite.Assert().Equal([]stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageClientAuth}, clientCert.ExtKeyUsage)

		frontProxyCert, err := kubernetesCerts.FrontProxy.GetCert()
		suite.Require().NoError(err)

		suite.Assert().Empty(frontProxyCert.DNSNames)
		suite.Assert().Empty(frontProxyCert.IPAddresses)

		suite.Assert().Equal("front-proxy-client", frontProxyCert.Subject.CommonName)
		suite.Assert().Empty(frontProxyCert.Subject.Organization)

		suite.Assert().Equal(
			stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment,
			frontProxyCert.KeyUsage,
		)
		suite.Assert().Equal(
			[]stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageClientAuth},
			frontProxyCert.ExtKeyUsage,
		)

		for _, kubeconfig := range []string{
			kubernetesCerts.ControllerManagerKubeconfig,
			kubernetesCerts.SchedulerKubeconfig,
			kubernetesCerts.LocalhostAdminKubeconfig,
			kubernetesCerts.AdminKubeconfig,
		} {
			config, err := clientcmd.Load([]byte(kubeconfig))
			suite.Require().NoError(err)

			suite.Assert().NoError(clientcmd.ConfirmUsable(*config, config.CurrentContext))
		}

		return nil
	})
}
