// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package secrets_test

import (
	"context"
	"fmt"
	"log"
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
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	"github.com/talos-systems/talos/pkg/machinery/resources/secrets"
)

type APICertSANsSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *APICertSANsSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&secretsctrl.APICertSANsController{}))

	suite.startRuntime()
}

func (suite *APICertSANsSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *APICertSANsSuite) TestReconcileControlPlane() {
	rootSecrets := secrets.NewOSRoot(secrets.OSRootID)

	rootSecrets.TypedSpec().CertSANDNSNames = []string{"some.org"}
	rootSecrets.TypedSpec().CertSANIPs = []netaddr.IP{netaddr.MustParseIP("10.4.3.2"), netaddr.MustParseIP("10.2.1.3")}
	suite.Require().NoError(suite.state.Create(suite.ctx, rootSecrets))

	hostnameStatus := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostnameStatus.TypedSpec().Hostname = "bar"
	hostnameStatus.TypedSpec().Domainname = "some.org"
	suite.Require().NoError(suite.state.Create(suite.ctx, hostnameStatus))

	nodeAddresses := network.NewNodeAddress(
		network.NamespaceName,
		network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, k8s.NodeAddressFilterNoK8s),
	)
	nodeAddresses.TypedSpec().Addresses = []netaddr.IPPrefix{
		netaddr.MustParseIPPrefix("10.2.1.3/24"),
		netaddr.MustParseIPPrefix("172.16.0.1/32"),
	}
	suite.Require().NoError(suite.state.Create(suite.ctx, nodeAddresses))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				certSANs, err := suite.state.Get(
					suite.ctx,
					resource.NewMetadata(
						secrets.NamespaceName,
						secrets.CertSANType,
						secrets.CertSANAPIID,
						resource.VersionUndefined,
					),
				)
				if err != nil {
					if state.IsNotFoundError(err) {
						return retry.ExpectedError(err)
					}

					return err
				}

				spec := certSANs.(*secrets.CertSAN).TypedSpec()

				suite.Assert().Equal([]string{"bar", "bar.some.org", "some.org"}, spec.DNSNames)
				suite.Assert().Equal("[10.2.1.3 10.4.3.2 172.16.0.1]", fmt.Sprintf("%v", spec.IPs))
				suite.Assert().Equal("bar.some.org", spec.FQDN)

				return nil
			},
		),
	)
}

func (suite *APICertSANsSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestAPICertSANsSuite(t *testing.T) {
	suite.Run(t, new(APICertSANsSuite))
}
