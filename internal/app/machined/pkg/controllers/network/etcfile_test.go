// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"context"
	"errors"
	"log"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/logging"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
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
	dnsServer      *network.DNSResolveCache

	podResolvConfPath string
}

func (suite *EtcFileConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.startRuntime()

	suite.podResolvConfPath = filepath.Join(suite.T().TempDir(), "resolv.conf")

	suite.Assert().NoFileExists(suite.podResolvConfPath)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.EtcFileController{
		PodResolvConfPath: suite.podResolvConfPath,
	}))

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	suite.cfg = config.NewMachineConfig(
		container.NewV1Alpha1(
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
		),
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

	suite.dnsServer = network.NewDNSResolveCache("udp")
	suite.dnsServer.TypedSpec().Status = "running"
}

func (suite *EtcFileConfigSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

type etcFileContents struct {
	hosts            string
	resolvConf       string
	resolvGlobalConf string
}

//nolint:gocyclo
func (suite *EtcFileConfigSuite) testFiles(resources []resource.Resource, contents etcFileContents) {
	for _, r := range resources {
		suite.Require().NoError(suite.state.Create(suite.ctx, r))
	}

	var (
		expectedIDs   []string
		unexpectedIDs []string
	)

	if contents.resolvConf != "" {
		expectedIDs = append(expectedIDs, "resolv.conf")
	} else {
		unexpectedIDs = append(unexpectedIDs, "resolv.conf")
	}

	if contents.hosts != "" {
		expectedIDs = append(expectedIDs, "hosts")
	} else {
		unexpectedIDs = append(unexpectedIDs, "hosts")
	}

	assertResources(
		suite.ctx,
		suite.T(),
		suite.state,
		expectedIDs,
		func(r *files.EtcFileSpec, asrt *assert.Assertions) {
			switch r.Metadata().ID() {
			case "hosts":
				asrt.Equal(contents.hosts, string(r.TypedSpec().Contents))
			case "resolv.conf":
				asrt.Equal(contents.resolvConf, string(r.TypedSpec().Contents))
			}
		},
	)
	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
			if contents.resolvGlobalConf == "" {
				_, err := os.Lstat(suite.podResolvConfPath)

				switch {
				case err == nil:
					return retry.ExpectedErrorf("unexpected pod %s", suite.podResolvConfPath)
				case errors.Is(err, os.ErrNotExist):
					return nil
				default:
					return err
				}
			}

			file, err := os.ReadFile(suite.podResolvConfPath)

			switch {
			case errors.Is(err, os.ErrNotExist):
				return retry.ExpectedErrorf("missing pod %s", suite.podResolvConfPath)
			case err != nil:
				return err
			default:
				suite.Assert().Equal(contents.resolvGlobalConf, string(file))

				return nil
			}
		}),
	)

	for _, id := range unexpectedIDs {
		assertNoResource[*files.EtcFileSpec](suite.ctx, suite.T(), suite.state, id)
	}
}

func (suite *EtcFileConfigSuite) TestComplete() {
	suite.testFiles(
		[]resource.Resource{suite.cfg, suite.defaultAddress, suite.hostnameStatus, suite.resolverStatus, suite.dnsServer},
		etcFileContents{
			hosts:            "127.0.0.1   localhost\n33.11.22.44 foo.example.com foo\n::1         localhost ip6-localhost ip6-loopback\nff02::1     ip6-allnodes\nff02::2     ip6-allrouters\n10.0.0.1    a b\n10.0.0.2    c d\n", //nolint:lll
			resolvConf:       "nameserver 127.0.0.53\nnameserver ::1\n\nsearch example.com\n",
			resolvGlobalConf: "nameserver 1.1.1.1\nnameserver 2.2.2.2\nnameserver 3.3.3.3\n\nsearch example.com\n",
		},
	)
}

func (suite *EtcFileConfigSuite) TestNoExtraHosts() {
	suite.testFiles(
		[]resource.Resource{suite.defaultAddress, suite.hostnameStatus, suite.resolverStatus, suite.dnsServer},
		etcFileContents{
			hosts:            "127.0.0.1   localhost\n33.11.22.44 foo.example.com foo\n::1         localhost ip6-localhost ip6-loopback\nff02::1     ip6-allnodes\nff02::2     ip6-allrouters\n",
			resolvConf:       "nameserver 127.0.0.53\nnameserver ::1\n\nsearch example.com\n",
			resolvGlobalConf: "nameserver 1.1.1.1\nnameserver 2.2.2.2\nnameserver 3.3.3.3\n\nsearch example.com\n",
		},
	)
}

func (suite *EtcFileConfigSuite) TestNoSearchDomain() {
	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkDisableSearchDomain: pointer.To(true),
					},
				},
			},
		),
	)
	suite.testFiles(
		[]resource.Resource{cfg, suite.defaultAddress, suite.hostnameStatus, suite.resolverStatus, suite.dnsServer},
		etcFileContents{
			hosts:            "127.0.0.1   localhost\n33.11.22.44 foo.example.com foo\n::1         localhost ip6-localhost ip6-loopback\nff02::1     ip6-allnodes\nff02::2     ip6-allrouters\n",
			resolvConf:       "nameserver 127.0.0.53\nnameserver ::1\n",
			resolvGlobalConf: "nameserver 1.1.1.1\nnameserver 2.2.2.2\nnameserver 3.3.3.3\n",
		},
	)
}

func (suite *EtcFileConfigSuite) TestNoDomainname() {
	suite.hostnameStatus.TypedSpec().Domainname = ""

	suite.testFiles(
		[]resource.Resource{suite.defaultAddress, suite.hostnameStatus, suite.resolverStatus, suite.dnsServer},
		etcFileContents{
			hosts:            "127.0.0.1   localhost\n33.11.22.44 foo\n::1         localhost ip6-localhost ip6-loopback\nff02::1     ip6-allnodes\nff02::2     ip6-allrouters\n",
			resolvConf:       "nameserver 127.0.0.53\nnameserver ::1\n",
			resolvGlobalConf: "nameserver 1.1.1.1\nnameserver 2.2.2.2\nnameserver 3.3.3.3\n",
		},
	)
}

func (suite *EtcFileConfigSuite) TestOnlyResolvers() {
	suite.testFiles(
		[]resource.Resource{suite.resolverStatus, suite.dnsServer},
		etcFileContents{
			hosts:            "",
			resolvConf:       "nameserver 127.0.0.53\nnameserver ::1\n",
			resolvGlobalConf: "nameserver 1.1.1.1\nnameserver 2.2.2.2\nnameserver 3.3.3.3\n",
		},
	)
}

func (suite *EtcFileConfigSuite) TestOnlyHostname() {
	suite.testFiles(
		[]resource.Resource{suite.defaultAddress, suite.hostnameStatus},
		etcFileContents{
			hosts:            "127.0.0.1   localhost\n33.11.22.44 foo.example.com foo\n::1         localhost ip6-localhost ip6-loopback\nff02::1     ip6-allnodes\nff02::2     ip6-allrouters\n",
			resolvConf:       "",
			resolvGlobalConf: "",
		},
	)
}

func (suite *EtcFileConfigSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	if _, err := os.Lstat(suite.podResolvConfPath); err == nil {
		suite.Require().NoError(os.Remove(suite.podResolvConfPath))
	}

	suite.wg.Wait()
}

func TestEtcFileConfigSuite(t *testing.T) {
	suite.Run(t, new(EtcFileConfigSuite))
}
