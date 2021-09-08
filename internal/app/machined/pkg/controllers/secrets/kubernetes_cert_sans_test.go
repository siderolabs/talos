// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package secrets_test

import (
	"context"
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
	"github.com/talos-systems/go-retry/retry"
	"inet.af/netaddr"

	secretsctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/resources/k8s"
	"github.com/talos-systems/talos/pkg/resources/network"
	"github.com/talos-systems/talos/pkg/resources/secrets"
)

type KubernetesCertSANsSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *KubernetesCertSANsSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&secretsctrl.KubernetesCertSANsController{}))

	suite.startRuntime()
}

func (suite *KubernetesCertSANsSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *KubernetesCertSANsSuite) TestReconcile() {
	rootSecrets := secrets.NewRoot(secrets.RootKubernetesID)

	var err error

	rootSecrets.KubernetesSpec().CertSANs = []string{"example.com"}
	rootSecrets.KubernetesSpec().APIServerIPs = []net.IP{net.ParseIP("10.4.3.2"), net.ParseIP("10.2.1.3")}
	rootSecrets.KubernetesSpec().DNSDomain = "cluster.remote"
	rootSecrets.KubernetesSpec().Endpoint, err = url.Parse("https://some.url:6443/")
	suite.Require().NoError(err)

	suite.Require().NoError(suite.state.Create(suite.ctx, rootSecrets))

	hostnameStatus := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostnameStatus.TypedSpec().Hostname = "foo"
	hostnameStatus.TypedSpec().Domainname = "example.com"
	suite.Require().NoError(suite.state.Create(suite.ctx, hostnameStatus))

	nodeAddresses := network.NewNodeAddress(network.NamespaceName, network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, k8s.NodeAddressFilterNoK8s))
	nodeAddresses.TypedSpec().Addresses = []netaddr.IPPrefix{netaddr.MustParseIPPrefix("10.2.1.3/24"), netaddr.MustParseIPPrefix("172.16.0.1/32")}
	suite.Require().NoError(suite.state.Create(suite.ctx, nodeAddresses))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			certSANs, err := suite.state.Get(suite.ctx, resource.NewMetadata(secrets.NamespaceName, secrets.CertSANType, secrets.CertSANKubernetesID, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			spec := certSANs.(*secrets.CertSAN).TypedSpec()

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
				}, spec.DNSNames)
			suite.Assert().Equal("[10.2.1.3 10.4.3.2 172.16.0.1]", fmt.Sprintf("%v", spec.IPs))

			return nil
		},
	))
}

func (suite *KubernetesCertSANsSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestKubernetesCertSANsSuite(t *testing.T) {
	suite.Run(t, new(KubernetesCertSANsSuite))
}
