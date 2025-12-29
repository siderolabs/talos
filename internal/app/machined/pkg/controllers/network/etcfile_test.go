// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"context"
	"errors"
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
	"go.uber.org/zap/zaptest"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/mount/v3"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/xfs"
	"github.com/siderolabs/talos/pkg/xfs/opentree"
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
	hostDNSConfig  *network.HostDNSConfig

	bindMountTarget   string
	podResolvConfPath string
	etcRoot           xfs.Root
}

func (suite *EtcFileConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.startRuntime()

	ok, err := v1alpha1runtime.KernelCapabilities().OpentreeOnAnonymousFS()
	suite.Require().NoError(err)

	if ok {
		suite.etcRoot = &xfs.UnixRoot{FS: opentree.NewFromPath(suite.T().TempDir())}
		suite.bindMountTarget = suite.T().TempDir()
		suite.podResolvConfPath = filepath.Join(suite.bindMountTarget, "resolv.conf")
	} else {
		suite.etcRoot = &xfs.OSRoot{Shadow: suite.T().TempDir()}
		suite.podResolvConfPath = filepath.Join(suite.etcRoot.Source(), "resolv.conf")
	}

	suite.Require().NoError(suite.etcRoot.OpenFS())
	suite.Assert().NoFileExists(suite.podResolvConfPath)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.EtcFileController{
		V1Alpha1Mode:    v1alpha1runtime.ModeMetal,
		EtcRoot:         suite.etcRoot,
		BindMountTarget: suite.bindMountTarget,
	}))

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	suite.cfg = config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy config
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

	suite.hostDNSConfig = network.NewHostDNSConfig(network.HostDNSConfigID)
	suite.hostDNSConfig.TypedSpec().Enabled = true
	suite.hostDNSConfig.TypedSpec().ListenAddresses = []netip.AddrPort{
		netip.MustParseAddrPort("127.0.0.53:53"),
		netip.MustParseAddrPort("169.254.116.108:53"),
	}
	suite.hostDNSConfig.TypedSpec().ServiceHostDNSAddress = netip.MustParseAddr("169.254.116.108")
}

func (suite *EtcFileConfigSuite) startRuntime() {
	suite.wg.Go(func() {
		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	})
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
			case len(file) == 0:
				return retry.ExpectedErrorf("empty pod %s", suite.podResolvConfPath)
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
	suite.resolverStatus.TypedSpec().SearchDomains = []string{"foo.example.com"}

	suite.testFiles(
		[]resource.Resource{suite.cfg, suite.defaultAddress, suite.hostnameStatus, suite.resolverStatus, suite.hostDNSConfig},
		etcFileContents{
			hosts:            "127.0.0.1   localhost\n33.11.22.44 foo.example.com foo\n::1         localhost ip6-localhost ip6-loopback\nff02::1     ip6-allnodes\nff02::2     ip6-allrouters\n10.0.0.1    a b\n10.0.0.2    c d\n", //nolint:lll
			resolvConf:       "nameserver 127.0.0.53\n\nsearch foo.example.com\n",
			resolvGlobalConf: "nameserver 169.254.116.108\n\nsearch foo.example.com\n",
		},
	)
}

func (suite *EtcFileConfigSuite) TestNoExtraHosts() {
	suite.resolverStatus.TypedSpec().SearchDomains = []string{"foo.example.com"}

	suite.testFiles(
		[]resource.Resource{suite.defaultAddress, suite.hostnameStatus, suite.resolverStatus, suite.hostDNSConfig},
		etcFileContents{
			hosts:            "127.0.0.1   localhost\n33.11.22.44 foo.example.com foo\n::1         localhost ip6-localhost ip6-loopback\nff02::1     ip6-allnodes\nff02::2     ip6-allrouters\n",
			resolvConf:       "nameserver 127.0.0.53\n\nsearch foo.example.com\n",
			resolvGlobalConf: "nameserver 169.254.116.108\n\nsearch foo.example.com\n",
		},
	)
}

func (suite *EtcFileConfigSuite) TestNoSearchDomainLegacy() {
	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy config
						NetworkDisableSearchDomain: pointer.To(true),
					},
				},
			},
		),
	)
	suite.testFiles(
		[]resource.Resource{cfg, suite.defaultAddress, suite.hostnameStatus, suite.resolverStatus, suite.hostDNSConfig},
		etcFileContents{
			hosts:            "127.0.0.1   localhost\n33.11.22.44 foo.example.com foo\n::1         localhost ip6-localhost ip6-loopback\nff02::1     ip6-allnodes\nff02::2     ip6-allrouters\n",
			resolvConf:       "nameserver 127.0.0.53\n",
			resolvGlobalConf: "nameserver 169.254.116.108\n",
		},
	)
}

func (suite *EtcFileConfigSuite) TestNoSearchDomainNewStyle() {
	hc := networkcfg.NewResolverConfigV1Alpha1()
	hc.ResolverSearchDomains = networkcfg.SearchDomainsConfig{
		SearchDisableDefault: pointer.To(true),
	}

	ctr, err := container.New(hc)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)

	suite.testFiles(
		[]resource.Resource{cfg, suite.defaultAddress, suite.hostnameStatus, suite.resolverStatus, suite.hostDNSConfig},
		etcFileContents{
			hosts:            "127.0.0.1   localhost\n33.11.22.44 foo.example.com foo\n::1         localhost ip6-localhost ip6-loopback\nff02::1     ip6-allnodes\nff02::2     ip6-allrouters\n",
			resolvConf:       "nameserver 127.0.0.53\n",
			resolvGlobalConf: "nameserver 169.254.116.108\n",
		},
	)
}

func (suite *EtcFileConfigSuite) TestNoDomainname() {
	suite.hostnameStatus.TypedSpec().Domainname = ""

	suite.testFiles(
		[]resource.Resource{suite.defaultAddress, suite.hostnameStatus, suite.resolverStatus, suite.hostDNSConfig},
		etcFileContents{
			hosts:            "127.0.0.1   localhost\n33.11.22.44 foo\n::1         localhost ip6-localhost ip6-loopback\nff02::1     ip6-allnodes\nff02::2     ip6-allrouters\n",
			resolvConf:       "nameserver 127.0.0.53\n",
			resolvGlobalConf: "nameserver 169.254.116.108\n",
		},
	)
}

func (suite *EtcFileConfigSuite) TestOnlyResolvers() {
	suite.testFiles(
		[]resource.Resource{suite.resolverStatus, suite.hostDNSConfig},
		etcFileContents{
			hosts:            "",
			resolvConf:       "nameserver 127.0.0.53\n",
			resolvGlobalConf: "nameserver 169.254.116.108\n",
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
		if suite.etcRoot.FSType() == "os" {
			suite.Require().NoError(os.Remove(suite.podResolvConfPath))
		} else {
			suite.Require().NoError(mount.SafeUnmount(context.Background(), nil, suite.podResolvConfPath))
		}
	}

	suite.wg.Wait()

	if suite.etcRoot != nil {
		suite.Require().NoError(os.RemoveAll(suite.bindMountTarget))

		suite.etcRoot.Close() //nolint:errcheck
	}
}

func TestEtcFileConfigSuite(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("skipping test that requires root privileges")
	}

	suite.Run(t, new(EtcFileConfigSuite))
}
