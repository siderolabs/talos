// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"fmt"
	"log"
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
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/go-retry/retry"
	"inet.af/netaddr"

	netctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	runtimeres "github.com/talos-systems/talos/pkg/machinery/resources/runtime"
	"github.com/talos-systems/talos/pkg/machinery/resources/v1alpha1"
)

type PlatformConfigSuite struct {
	suite.Suite

	state state.State

	statePath string

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *PlatformConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.statePath = suite.T().TempDir()
}

func (suite *PlatformConfigSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *PlatformConfigSuite) assertResources(resourceNamespace resource.Namespace, resourceType resource.Type, requiredIDs []string, check func(resource.Resource) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(resourceNamespace, resourceType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		if _, ok := missingIDs[res.Metadata().ID()]; ok {
			if err = check(res); err != nil {
				return retry.ExpectedError(err)
			}
		}

		delete(missingIDs, res.Metadata().ID())
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
}

func (suite *PlatformConfigSuite) assertNoResource(resourceType resource.Type, id string) error {
	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.ConfigNamespaceName, resourceType, "", resource.VersionUndefined))
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

func (suite *PlatformConfigSuite) TestNoPlatform() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.PlatformConfigController{}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNoResource(network.HostnameSpecType, "platform/hostname")
		}))
}

func (suite *PlatformConfigSuite) TestPlatformMockHostname() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.PlatformConfigController{
		V1alpha1Platform: &platformMock{hostname: []byte("talos-e2e-897b4e49-gcp-controlplane-jvcnl.c.talos-testbed.internal")},
		StatePath:        suite.statePath,
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertResources(network.ConfigNamespaceName, network.HostnameSpecType, []string{
				"platform/hostname",
			}, func(r resource.Resource) error {
				spec := r.(*network.HostnameSpec).TypedSpec()

				suite.Assert().Equal("talos-e2e-897b4e49-gcp-controlplane-jvcnl", spec.Hostname)
				suite.Assert().Equal("c.talos-testbed.internal", spec.Domainname)
				suite.Assert().Equal(network.ConfigPlatform, spec.ConfigLayer)

				return nil
			})
		}))
}

func (suite *PlatformConfigSuite) TestPlatformMockHostnameNoDomain() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.PlatformConfigController{
		V1alpha1Platform: &platformMock{hostname: []byte("talos-e2e-897b4e49-gcp-controlplane-jvcnl")},
		StatePath:        suite.statePath,
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertResources(network.ConfigNamespaceName, network.HostnameSpecType, []string{
				"platform/hostname",
			}, func(r resource.Resource) error {
				spec := r.(*network.HostnameSpec).TypedSpec()

				suite.Assert().Equal("talos-e2e-897b4e49-gcp-controlplane-jvcnl", spec.Hostname)
				suite.Assert().Equal("", spec.Domainname)
				suite.Assert().Equal(network.ConfigPlatform, spec.ConfigLayer)

				return nil
			})
		}))
}

func (suite *PlatformConfigSuite) TestPlatformMockAddresses() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.PlatformConfigController{
		V1alpha1Platform: &platformMock{
			addresses: []netaddr.IPPrefix{netaddr.MustParseIPPrefix("192.168.1.24/24"), netaddr.MustParseIPPrefix("2001:fd::3/64")},
		},
		StatePath: suite.statePath,
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertResources(network.ConfigNamespaceName, network.AddressSpecType, []string{
				"platform/eth0/192.168.1.24/24",
				"platform/eth0/2001:fd::3/64",
			}, func(r resource.Resource) error {
				spec := r.(*network.AddressSpec).TypedSpec()

				switch r.Metadata().ID() {
				case "platform/eth0/192.168.1.24/24":
					suite.Assert().Equal(nethelpers.FamilyInet4, spec.Family)
					suite.Assert().Equal("192.168.1.24/24", spec.Address.String())
				case "platform/eth0/2001:fd::3/64":
					suite.Assert().Equal(nethelpers.FamilyInet6, spec.Family)
					suite.Assert().Equal("2001:fd::3/64", spec.Address.String())
				}

				suite.Assert().Equal(network.ConfigPlatform, spec.ConfigLayer)

				return nil
			})
		}))
}

func (suite *PlatformConfigSuite) TestPlatformMockLinks() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.PlatformConfigController{
		V1alpha1Platform: &platformMock{
			linksUp: []string{"eth0", "eth1"},
		},
		StatePath: suite.statePath,
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertResources(network.ConfigNamespaceName, network.LinkSpecType, []string{
				"platform/eth0",
				"platform/eth1",
			}, func(r resource.Resource) error {
				spec := r.(*network.LinkSpec).TypedSpec()

				suite.Assert().True(spec.Up)
				suite.Assert().Equal(network.ConfigPlatform, spec.ConfigLayer)

				return nil
			})
		}))
}

func (suite *PlatformConfigSuite) TestPlatformMockRoutes() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.PlatformConfigController{
		V1alpha1Platform: &platformMock{
			defaultRoutes: []netaddr.IP{netaddr.MustParseIP("10.0.0.1")},
		},
		StatePath: suite.statePath,
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertResources(network.ConfigNamespaceName, network.RouteSpecType, []string{
				"platform/inet4/10.0.0.1//1024",
			}, func(r resource.Resource) error {
				spec := r.(*network.RouteSpec).TypedSpec()

				suite.Assert().Equal("10.0.0.1", spec.Gateway.String())
				suite.Assert().Equal(network.ConfigPlatform, spec.ConfigLayer)

				return nil
			})
		}))
}

func (suite *PlatformConfigSuite) TestPlatformMockOperators() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.PlatformConfigController{
		V1alpha1Platform: &platformMock{
			dhcp4Links: []string{"eth1", "eth2"},
		},
		StatePath: suite.statePath,
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertResources(network.ConfigNamespaceName, network.OperatorSpecType, []string{
				"platform/dhcp4/eth1",
				"platform/dhcp4/eth2",
			}, func(r resource.Resource) error {
				spec := r.(*network.OperatorSpec).TypedSpec()

				suite.Assert().Equal(network.OperatorDHCP4, spec.Operator)
				suite.Assert().Equal(network.ConfigPlatform, spec.ConfigLayer)

				return nil
			})
		}))
}

func (suite *PlatformConfigSuite) TestPlatformMockResolvers() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.PlatformConfigController{
		V1alpha1Platform: &platformMock{
			resolvers: []netaddr.IP{netaddr.MustParseIP("1.1.1.1")},
		},
		StatePath: suite.statePath,
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertResources(network.ConfigNamespaceName, network.ResolverSpecType, []string{
				"platform/resolvers",
			}, func(r resource.Resource) error {
				spec := r.(*network.ResolverSpec).TypedSpec()

				suite.Assert().Equal("[1.1.1.1]", fmt.Sprintf("%s", spec.DNSServers))
				suite.Assert().Equal(network.ConfigPlatform, spec.ConfigLayer)

				return nil
			})
		}))
}

func (suite *PlatformConfigSuite) TestPlatformMockTimeServers() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.PlatformConfigController{
		V1alpha1Platform: &platformMock{
			timeServers: []string{"pool.ntp.org"},
		},
		StatePath: suite.statePath,
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertResources(network.ConfigNamespaceName, network.TimeServerSpecType, []string{
				"platform/timeservers",
			}, func(r resource.Resource) error {
				spec := r.(*network.TimeServerSpec).TypedSpec()

				suite.Assert().Equal("[pool.ntp.org]", fmt.Sprintf("%s", spec.NTPServers))
				suite.Assert().Equal(network.ConfigPlatform, spec.ConfigLayer)

				return nil
			})
		}))
}

func (suite *PlatformConfigSuite) TestPlatformMockExternalIPs() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.PlatformConfigController{
		V1alpha1Platform: &platformMock{externalIPs: []netaddr.IP{netaddr.MustParseIP("10.3.4.5"), netaddr.MustParseIP("2001:470:6d:30e:96f4:4219:5733:b860")}},
		StatePath:        suite.statePath,
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertResources(network.NamespaceName, network.AddressStatusType, []string{
				"external/10.3.4.5/32",
				"external/2001:470:6d:30e:96f4:4219:5733:b860/128",
			}, func(r resource.Resource) error {
				spec := r.(*network.AddressStatus).TypedSpec()

				suite.Assert().Equal("external", spec.LinkName)
				suite.Assert().Equal(nethelpers.ScopeGlobal, spec.Scope)

				if r.Metadata().ID() == "external/10.3.4.5/32" {
					suite.Assert().Equal(nethelpers.FamilyInet4, spec.Family)
				} else {
					suite.Assert().Equal(nethelpers.FamilyInet6, spec.Family)
				}

				return nil
			})
		}))
}

const sampleStoredConfig = "addresses: []\nlinks: []\nroutes: []\nhostnames:\n    - hostname: talos-e2e-897b4e49-gcp-controlplane-jvcnl\n      domainname: \"\"\n      layer: default\nresolvers: []\ntimeServers: []\noperators: []\nexternalIPs:\n    - 10.3.4.5\n    - 2001:470:6d:30e:96f4:4219:5733:b860\n" //nolint:lll

func (suite *PlatformConfigSuite) TestStoreConfig() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.PlatformConfigController{
		V1alpha1Platform: &platformMock{
			hostname:    []byte("talos-e2e-897b4e49-gcp-controlplane-jvcnl"),
			externalIPs: []netaddr.IP{netaddr.MustParseIP("10.3.4.5"), netaddr.MustParseIP("2001:470:6d:30e:96f4:4219:5733:b860")},
		},
		StatePath: suite.statePath,
	}))

	suite.startRuntime()

	stateMount := runtimeres.NewMountStatus(v1alpha1.NamespaceName, constants.StatePartitionLabel)

	suite.Assert().NoError(suite.state.Create(suite.ctx, stateMount))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			contents, err := os.ReadFile(filepath.Join(suite.statePath, constants.PlatformNetworkConfigFilename))
			if err != nil {
				if os.IsNotExist(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			suite.Assert().Equal(sampleStoredConfig, string(contents))

			return nil
		}))
}

func (suite *PlatformConfigSuite) TestLoadConfig() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.PlatformConfigController{
		V1alpha1Platform: &platformMock{
			noData: true,
		},
		StatePath: suite.statePath,
	}))

	suite.startRuntime()

	suite.Require().NoError(os.WriteFile(filepath.Join(suite.statePath, constants.PlatformNetworkConfigFilename), []byte(sampleStoredConfig), 0o400))

	stateMount := runtimeres.NewMountStatus(v1alpha1.NamespaceName, constants.StatePartitionLabel)

	suite.Assert().NoError(suite.state.Create(suite.ctx, stateMount))

	// controller should pick up cached network configuration
	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertResources(network.NamespaceName, network.AddressStatusType, []string{
				"external/10.3.4.5/32",
				"external/2001:470:6d:30e:96f4:4219:5733:b860/128",
			}, func(r resource.Resource) error {
				spec := r.(*network.AddressStatus).TypedSpec()

				suite.Assert().Equal("external", spec.LinkName)
				suite.Assert().Equal(nethelpers.ScopeGlobal, spec.Scope)

				if r.Metadata().ID() == "external/10.3.4.5/32" {
					suite.Assert().Equal(nethelpers.FamilyInet4, spec.Family)
				} else {
					suite.Assert().Equal(nethelpers.FamilyInet6, spec.Family)
				}

				return nil
			})
		}))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertResources(network.ConfigNamespaceName, network.HostnameSpecType, []string{
				"platform/hostname",
			}, func(r resource.Resource) error {
				spec := r.(*network.HostnameSpec).TypedSpec()

				suite.Assert().Equal("talos-e2e-897b4e49-gcp-controlplane-jvcnl", spec.Hostname)
				suite.Assert().Equal("", spec.Domainname)
				suite.Assert().Equal(network.ConfigPlatform, spec.ConfigLayer)

				return nil
			})
		}))
}

func (suite *PlatformConfigSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestPlatformConfigSuite(t *testing.T) {
	suite.Run(t, new(PlatformConfigSuite))
}

type platformMock struct {
	noData bool

	hostname      []byte
	externalIPs   []netaddr.IP
	addresses     []netaddr.IPPrefix
	defaultRoutes []netaddr.IP
	linksUp       []string
	resolvers     []netaddr.IP
	timeServers   []string
	dhcp4Links    []string
}

func (mock *platformMock) Name() string {
	return "mock"
}

func (mock *platformMock) Configuration(context.Context) ([]byte, error) {
	return nil, nil
}

func (mock *platformMock) Mode() v1alpha1runtime.Mode {
	return v1alpha1runtime.ModeCloud
}

func (mock *platformMock) KernelArgs() procfs.Parameters {
	return nil
}

//nolint:gocyclo
func (mock *platformMock) NetworkConfiguration(ctx context.Context, ch chan<- *v1alpha1runtime.PlatformNetworkConfig) error {
	if mock.noData {
		return nil
	}

	networkConfig := &v1alpha1runtime.PlatformNetworkConfig{
		ExternalIPs: mock.externalIPs,
	}

	if mock.hostname != nil {
		hostnameSpec := network.HostnameSpecSpec{}
		if err := hostnameSpec.ParseFQDN(string(mock.hostname)); err != nil {
			return err
		}

		networkConfig.Hostnames = []network.HostnameSpecSpec{hostnameSpec}
	}

	for _, addr := range mock.addresses {
		family := nethelpers.FamilyInet4
		if addr.IP().Is6() {
			family = nethelpers.FamilyInet6
		}

		networkConfig.Addresses = append(networkConfig.Addresses,
			network.AddressSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				LinkName:    "eth0",
				Address:     addr,
				Scope:       nethelpers.ScopeGlobal,
				Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
				Family:      family,
			})
	}

	for _, gw := range mock.defaultRoutes {
		family := nethelpers.FamilyInet4
		if gw.Is6() {
			family = nethelpers.FamilyInet6
		}

		route := network.RouteSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			Gateway:     gw,
			OutLinkName: "eth0",
			Table:       nethelpers.TableMain,
			Protocol:    nethelpers.ProtocolStatic,
			Type:        nethelpers.TypeUnicast,
			Family:      family,
			Priority:    1024,
		}

		route.Normalize()

		networkConfig.Routes = append(networkConfig.Routes, route)
	}

	for _, link := range mock.linksUp {
		networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			Name:        link,
			Up:          true,
		})
	}

	if len(mock.resolvers) > 0 {
		networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			DNSServers:  mock.resolvers,
		})
	}

	if len(mock.timeServers) > 0 {
		networkConfig.TimeServers = append(networkConfig.TimeServers, network.TimeServerSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			NTPServers:  mock.timeServers,
		})
	}

	for _, link := range mock.dhcp4Links {
		networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			LinkName:    link,
			Operator:    network.OperatorDHCP4,
			DHCP4:       network.DHCP4OperatorSpec{},
		})
	}

	select {
	case ch <- networkConfig:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
