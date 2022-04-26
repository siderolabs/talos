// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package secrets_test

import (
	"context"
	stdlibx509 "crypto/x509"
	"fmt"
	"log"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/crypto/x509"
	"github.com/talos-systems/go-retry/retry"
	"k8s.io/client-go/tools/clientcmd"

	secretsctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	"github.com/talos-systems/talos/pkg/machinery/resources/secrets"
	timeresource "github.com/talos-systems/talos/pkg/machinery/resources/time"
)

type KubernetesSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *KubernetesSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&secretsctrl.KubernetesController{}))

	suite.startRuntime()
}

func (suite *KubernetesSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
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
	suite.Require().NoError(suite.state.Create(suite.ctx, rootSecrets))

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeControlPlane)
	suite.Require().NoError(suite.state.Create(suite.ctx, machineType))

	networkStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	networkStatus.TypedSpec().AddressReady = true
	networkStatus.TypedSpec().HostnameReady = true
	suite.Require().NoError(suite.state.Create(suite.ctx, networkStatus))

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
	suite.Require().NoError(suite.state.Create(suite.ctx, certSANs))

	timeSync := timeresource.NewStatus()
	*timeSync.TypedSpec() = timeresource.StatusSpec{
		Synced: true,
	}
	suite.Require().NoError(suite.state.Create(suite.ctx, timeSync))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				certs, err := suite.state.Get(
					suite.ctx,
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

				kubernetesCerts := certs.(*secrets.Kubernetes).TypedSpec()

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
			},
		),
	)
}

func (suite *KubernetesSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestKubernetesSuite(t *testing.T) {
	suite.Run(t, new(KubernetesSuite))
}
