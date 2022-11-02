// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/logging"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type EtcFileConfigSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	cfg            *config.MachineConfig
	defaultAddress *network.NodeAddress
	hostnameStatus *network.HostnameStatus
	resolverStatus *network.ResolverStatus
}

func (suite *EtcFileConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.EtcFileController{}))

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	suite.cfg = config.NewMachineConfig(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{
				MachineNetwork: &v1alpha1.NetworkConfig{
					ExtraHostEntries: []*v1alpha1.ExtraHost{
						{
							HostIP:      "10.0.0.1",
							HostAliases: []string{"a", "b"},
						},
						{
							HostIP:      "10.0.0.2",
							HostAliases: []string{"c", "d"},
						},
					},
				},
			},
			ClusterConfig: &v1alpha1.ClusterConfig{
				ControlPlane: &v1alpha1.ControlPlaneConfig{
					Endpoint: &v1alpha1.Endpoint{
						URL: u,
					},
				},
			},
		},
	)

	suite.defaultAddress = network.NewNodeAddress(network.NamespaceName, network.NodeAddressDefaultID)
	suite.defaultAddress.TypedSpec().Addresses = []netip.Prefix{netip.MustParsePrefix("33.11.22.44/32")}

	suite.hostnameStatus = network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	suite.hostnameStatus.TypedSpec().Hostname = "foo"
	suite.hostnameStatus.TypedSpec().Domainname = "example.com"

	suite.resolverStatus = network.NewResolverStatus(network.NamespaceName, network.ResolverID)
	suite.resolverStatus.TypedSpec().DNSServers = []netip.Addr{
		netip.MustParseAddr("1.1.1.1"),
		netip.MustParseAddr("2.2.2.2"),
		netip.MustParseAddr("3.3.3.3"),
		netip.MustParseAddr("4.4.4.4"),
	}
}

func (suite *EtcFileConfigSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *EtcFileConfigSuite) assertEtcFiles(requiredIDs []string, check func(*files.EtcFileSpec) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(files.NamespaceName, files.EtcFileSpecType, "", resource.VersionUndefined),
	)
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		_, required := missingIDs[res.Metadata().ID()]
		if !required {
			continue
		}

		delete(missingIDs, res.Metadata().ID())

		if err = check(res.(*files.EtcFileSpec)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
}

func (suite *EtcFileConfigSuite) assertNoEtcFile(id string) error {
	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(files.NamespaceName, files.EtcFileSpecType, "", resource.VersionUndefined),
	)
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		if res.Metadata().ID() == id {
			return retry.ExpectedError(fmt.Errorf("spec %q is still there", id))
		}
	}

	return nil
}

func (suite *EtcFileConfigSuite) testFiles(resources []resource.Resource, resolvConf, hosts string) {
	for _, r := range resources {
		suite.Require().NoError(suite.state.Create(suite.ctx, r))
	}

	expectedIds, unexpectedIds := []string{}, []string{}

	if resolvConf != "" {
		expectedIds = append(expectedIds, "resolv.conf")
	} else {
		unexpectedIds = append(unexpectedIds, "resolv.conf")
	}

	if hosts != "" {
		expectedIds = append(expectedIds, "hosts")
	} else {
		unexpectedIds = append(unexpectedIds, "hosts")
	}

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertEtcFiles(
					expectedIds,
					func(r *files.EtcFileSpec) error {
						switch r.Metadata().ID() {
						case "hosts":
							suite.Assert().Equal(hosts, string(r.TypedSpec().Contents))
						case "resolv.conf":
							suite.Assert().Equal(resolvConf, string(r.TypedSpec().Contents))
						}

						return nil
					},
				)
			},
		),
	)

	for _, id := range unexpectedIds {
		id := id

		suite.Assert().NoError(
			retry.Constant(1*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
				func() error {
					return suite.assertNoEtcFile(id)
				},
			),
		)
	}
}

func (suite *EtcFileConfigSuite) TestComplete() {
	suite.testFiles(
		[]resource.Resource{suite.cfg, suite.defaultAddress, suite.hostnameStatus, suite.resolverStatus},
		"nameserver 1.1.1.1\nnameserver 2.2.2.2\nnameserver 3.3.3.3\n\nsearch example.com\n",
		"127.0.0.1   localhost\n33.11.22.44 foo.example.com foo\n::1         localhost ip6-localhost ip6-loopback\nff02::1     ip6-allnodes\nff02::2     ip6-allrouters\n10.0.0.1    a b\n10.0.0.2    c d\n", //nolint:lll
	)
}

func (suite *EtcFileConfigSuite) TestNoExtraHosts() {
	suite.testFiles(
		[]resource.Resource{suite.defaultAddress, suite.hostnameStatus, suite.resolverStatus},
		"nameserver 1.1.1.1\nnameserver 2.2.2.2\nnameserver 3.3.3.3\n\nsearch example.com\n",
		"127.0.0.1   localhost\n33.11.22.44 foo.example.com foo\n::1         localhost ip6-localhost ip6-loopback\nff02::1     ip6-allnodes\nff02::2     ip6-allrouters\n",
	)
}

func (suite *EtcFileConfigSuite) TestNoSearchDomain() {
	cfg := config.NewMachineConfig(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{
				MachineNetwork: &v1alpha1.NetworkConfig{
					NetworkDisableSearchDomain: pointer.To(true),
				},
			},
		},
	)
	suite.testFiles(
		[]resource.Resource{cfg, suite.defaultAddress, suite.hostnameStatus, suite.resolverStatus},
		"nameserver 1.1.1.1\nnameserver 2.2.2.2\nnameserver 3.3.3.3\n",
		"127.0.0.1   localhost\n33.11.22.44 foo.example.com foo\n::1         localhost ip6-localhost ip6-loopback\nff02::1     ip6-allnodes\nff02::2     ip6-allrouters\n", //nolint:lll
	)
}

func (suite *EtcFileConfigSuite) TestNoDomainname() {
	suite.hostnameStatus.TypedSpec().Domainname = ""

	suite.testFiles(
		[]resource.Resource{suite.defaultAddress, suite.hostnameStatus, suite.resolverStatus},
		"nameserver 1.1.1.1\nnameserver 2.2.2.2\nnameserver 3.3.3.3\n",
		"127.0.0.1   localhost\n33.11.22.44 foo\n::1         localhost ip6-localhost ip6-loopback\nff02::1     ip6-allnodes\nff02::2     ip6-allrouters\n",
	)
}

func (suite *EtcFileConfigSuite) TestOnlyResolvers() {
	suite.testFiles(
		[]resource.Resource{suite.resolverStatus},
		"nameserver 1.1.1.1\nnameserver 2.2.2.2\nnameserver 3.3.3.3\n",
		"",
	)
}

func (suite *EtcFileConfigSuite) TestOnlyHostname() {
	suite.testFiles(
		[]resource.Resource{suite.defaultAddress, suite.hostnameStatus},
		"",
		"127.0.0.1   localhost\n33.11.22.44 foo.example.com foo\n::1         localhost ip6-localhost ip6-loopback\nff02::1     ip6-allnodes\nff02::2     ip6-allrouters\n",
	)
}

func (suite *EtcFileConfigSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	err := suite.state.Create(
		context.Background(), config.NewMachineConfig(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
			},
		),
	)
	if state.IsConflictError(err) {
		err = suite.state.Destroy(context.Background(), config.NewMachineConfig(nil).Metadata())
	}

	suite.Require().NoError(err)

	suite.Assert().NoError(
		suite.state.Create(
			context.Background(),
			network.NewHostnameStatus(network.NamespaceName, "bar"),
		),
	)
	suite.Assert().NoError(
		suite.state.Create(
			context.Background(),
			network.NewResolverStatus(network.NamespaceName, "bar"),
		),
	)
	suite.Assert().NoError(
		suite.state.Create(
			context.Background(),
			network.NewNodeAddress(network.NamespaceName, "bar"),
		),
	)
}

func TestEtcFileConfigSuite(t *testing.T) {
	suite.Run(t, new(EtcFileConfigSuite))
}
