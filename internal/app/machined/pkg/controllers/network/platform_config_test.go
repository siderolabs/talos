// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type PlatformConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *PlatformConfigSuite) TestNoPlatform() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.PlatformConfigController{}))

	ctest.AssertNoResource[*network.HostnameSpec](suite, "platform/hostname", rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigSuite) TestPlatformMockHostname() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.PlatformConfigController{
				V1alpha1Platform: &platformMock{hostname: []byte("talos-e2e-897b4e49-gcp-controlplane-jvcnl.c.talos-testbed.internal")},
			},
		),
	)

	ctest.AssertResource(suite, "platform/hostname", func(hostname *network.HostnameSpec, asrt *assert.Assertions) {
		spec := hostname.TypedSpec()

		asrt.Equal("talos-e2e-897b4e49-gcp-controlplane-jvcnl", spec.Hostname)
		asrt.Equal("c.talos-testbed.internal", spec.Domainname)
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigSuite) TestPlatformMockHostnameNoDomain() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.PlatformConfigController{
				V1alpha1Platform: &platformMock{hostname: []byte("talos-e2e-897b4e49-gcp-controlplane-jvcnl")},
				PlatformState:    suite.State(),
			},
		),
	)

	ctest.AssertResource(suite, "platform/hostname", func(hostname *network.HostnameSpec, asrt *assert.Assertions) {
		spec := hostname.TypedSpec()

		asrt.Equal("talos-e2e-897b4e49-gcp-controlplane-jvcnl", spec.Hostname)
		asrt.Equal("", spec.Domainname)
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigSuite) TestPlatformMockAddresses() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.PlatformConfigController{
				V1alpha1Platform: &platformMock{
					addresses: []netip.Prefix{
						netip.MustParsePrefix("192.168.1.24/24"),
						netip.MustParsePrefix("2001:fd::3/64"),
					},
				},
				PlatformState: suite.State(),
			},
		),
	)

	ctest.AssertResources(suite, []string{
		"platform/eth0/192.168.1.24/24",
		"platform/eth0/2001:fd::3/64",
	}, func(r *network.AddressSpec, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		switch r.Metadata().ID() {
		case "platform/eth0/192.168.1.24/24":
			asrt.Equal(nethelpers.FamilyInet4, spec.Family)
			asrt.Equal("192.168.1.24/24", spec.Address.String())
		case "platform/eth0/2001:fd::3/64":
			asrt.Equal(nethelpers.FamilyInet6, spec.Family)
			asrt.Equal("2001:fd::3/64", spec.Address.String())
		}

		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigSuite) TestPlatformMockLinks() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.PlatformConfigController{
				V1alpha1Platform: &platformMock{
					linksUp: []string{"eth0", "eth1"},
				},
				PlatformState: suite.State(),
			},
		),
	)

	ctest.AssertResources(suite, []string{
		"platform/eth0",
		"platform/eth1",
	}, func(r *network.LinkSpec, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.True(spec.Up)
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigSuite) TestPlatformMockRoutes() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.PlatformConfigController{
				V1alpha1Platform: &platformMock{
					defaultRoutes: []netip.Addr{netip.MustParseAddr("10.0.0.1")},
				},
				PlatformState: suite.State(),
			},
		),
	)

	ctest.AssertResources(suite, []string{
		"platform/inet4/10.0.0.1//1024",
	}, func(r *network.RouteSpec, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal("10.0.0.1", spec.Gateway.String())
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigSuite) TestPlatformMockOperators() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.PlatformConfigController{
				V1alpha1Platform: &platformMock{
					dhcp4Links: []string{"eth1", "eth2"},
				},
				PlatformState: suite.State(),
			},
		),
	)

	ctest.AssertResources(suite, []string{
		"platform/dhcp4/eth1",
		"platform/dhcp4/eth2",
	}, func(r *network.OperatorSpec, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal(network.OperatorDHCP4, spec.Operator)
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigSuite) TestPlatformMockResolvers() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.PlatformConfigController{
				V1alpha1Platform: &platformMock{
					resolvers: []netip.Addr{netip.MustParseAddr("1.1.1.1")},
				},
				PlatformState: suite.State(),
			},
		),
	)

	ctest.AssertResources(suite, []string{
		"platform/resolvers",
	}, func(r *network.ResolverSpec, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal("[1.1.1.1]", fmt.Sprintf("%s", spec.DNSServers))
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigSuite) TestPlatformMockTimeServers() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.PlatformConfigController{
				V1alpha1Platform: &platformMock{
					timeServers: []string{"pool.ntp.org"},
				},
				PlatformState: suite.State(),
			},
		),
	)

	ctest.AssertResources(suite, []string{
		"platform/timeservers",
	}, func(r *network.TimeServerSpec, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal("[pool.ntp.org]", fmt.Sprintf("%s", spec.NTPServers))
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigSuite) TestPlatformMockProbes() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.PlatformConfigController{
				V1alpha1Platform: &platformMock{
					tcpProbes: []string{"example.com:80", "example.com:443"},
				},
				PlatformState: suite.State(),
			},
		),
	)

	ctest.AssertResources(suite, []string{
		"tcp:example.com:80",
		"tcp:example.com:443",
	}, func(r *network.ProbeSpec, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal(time.Second, spec.Interval)
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	})
}

func (suite *PlatformConfigSuite) TestPlatformMockExternalIPs() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.PlatformConfigController{
				V1alpha1Platform: &platformMock{
					externalIPs: []netip.Addr{
						netip.MustParseAddr("10.3.4.5"),
						netip.MustParseAddr("2001:470:6d:30e:96f4:4219:5733:b860"),
					},
				},
				PlatformState: suite.State(),
			},
		),
	)

	ctest.AssertResources(suite, []string{
		"external/10.3.4.5/32",
		"external/2001:470:6d:30e:96f4:4219:5733:b860/128",
	}, func(r *network.AddressStatus, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal("external", spec.LinkName)
		asrt.Equal(nethelpers.ScopeGlobal, spec.Scope)

		if r.Metadata().ID() == "external/10.3.4.5/32" {
			asrt.Equal(nethelpers.FamilyInet4, spec.Family)
		} else {
			asrt.Equal(nethelpers.FamilyInet6, spec.Family)
		}
	})
}

func (suite *PlatformConfigSuite) TestPlatformMockMetadata() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.PlatformConfigController{
				V1alpha1Platform: &platformMock{
					metadata: &runtimeres.PlatformMetadataSpec{
						Platform: "mock",
						Zone:     "mock-zone",
					},
				},
				PlatformState: suite.State(),
			},
		),
	)

	ctest.AssertResource(suite, runtimeres.PlatformMetadataID,
		func(r *runtimeres.PlatformMetadata, asrt *assert.Assertions) {
			asrt.Equal("mock", r.TypedSpec().Platform)
			asrt.Equal("mock-zone", r.TypedSpec().Zone)
		})
}

const sampleStoredConfig = "addresses: []\nlinks: []\nroutes: []\nhostnames:\n    - hostname: talos-e2e-897b4e49-gcp-controlplane-jvcnl\n      domainname: \"\"\n      layer: default\nresolvers: []\ntimeServers: []\noperators: []\nexternalIPs:\n    - 10.3.4.5\n    - 2001:470:6d:30e:96f4:4219:5733:b860\n" //nolint:lll

func (suite *PlatformConfigSuite) TestStoreConfig() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.PlatformConfigController{
				V1alpha1Platform: &platformMock{
					hostname: []byte("talos-e2e-897b4e49-gcp-controlplane-jvcnl"),
					externalIPs: []netip.Addr{
						netip.MustParseAddr("10.3.4.5"),
						netip.MustParseAddr("2001:470:6d:30e:96f4:4219:5733:b860"),
					},
				},
				PlatformState: suite.State(),
			},
		),
	)

	// wait for the controller to acquire the config
	ctest.AssertResources(suite, []string{
		"external/10.3.4.5/32",
	}, func(r *network.AddressStatus, asrt *assert.Assertions) {})

	statePath := suite.T().TempDir()
	mountID := (&netctrl.PlatformConfigController{}).Name() + "-" + constants.StatePartitionLabel

	ctest.AssertResource(suite, mountID, func(mountRequest *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(constants.StatePartitionLabel, mountRequest.TypedSpec().VolumeID)
	})

	volumeMountStatus := block.NewVolumeMountStatus(block.NamespaceName, mountID)
	volumeMountStatus.TypedSpec().Target = statePath
	suite.Create(volumeMountStatus)

	suite.EventuallyWithT(func(collect *assert.CollectT) {
		asrt := assert.New(collect)

		contents, err := os.ReadFile(filepath.Join(statePath, constants.PlatformNetworkConfigFilename))
		asrt.NoError(err)

		asrt.Equal(sampleStoredConfig, string(contents))
	}, time.Second, 10*time.Millisecond)

	ctest.AssertResources(suite, []resource.ID{volumeMountStatus.Metadata().ID()}, func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(vms.Metadata().Finalizers().Empty())
	})

	suite.Destroy(volumeMountStatus)

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, mountID)
}

func (suite *PlatformConfigSuite) TestLoadConfig() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.PlatformConfigController{
				V1alpha1Platform: &platformMock{
					noData: true,
				},
				PlatformState: suite.State(),
			},
		),
	)

	statePath := suite.T().TempDir()
	mountID := (&netctrl.PlatformConfigController{}).Name() + "-" + constants.StatePartitionLabel

	suite.Require().NoError(
		os.WriteFile(
			filepath.Join(statePath, constants.PlatformNetworkConfigFilename),
			[]byte(sampleStoredConfig),
			0o400,
		),
	)

	ctest.AssertResource(suite, mountID, func(mountRequest *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(constants.StatePartitionLabel, mountRequest.TypedSpec().VolumeID)
	})

	volumeMountStatus := block.NewVolumeMountStatus(block.NamespaceName, mountID)
	volumeMountStatus.TypedSpec().Target = statePath
	suite.Create(volumeMountStatus)

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, mountID)

	suite.Destroy(volumeMountStatus)

	// controller should pick up cached network configuration
	ctest.AssertResources(suite, []string{
		"external/10.3.4.5/32",
		"external/2001:470:6d:30e:96f4:4219:5733:b860/128",
	}, func(r *network.AddressStatus, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal("external", spec.LinkName)
		asrt.Equal(nethelpers.ScopeGlobal, spec.Scope)

		if r.Metadata().ID() == "external/10.3.4.5/32" {
			asrt.Equal(nethelpers.FamilyInet4, spec.Family)
		} else {
			asrt.Equal(nethelpers.FamilyInet6, spec.Family)
		}
	})

	ctest.AssertResources(suite, []string{
		"platform/hostname",
	}, func(r *network.HostnameSpec, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal("talos-e2e-897b4e49-gcp-controlplane-jvcnl", spec.Hostname)
		asrt.Equal("", spec.Domainname)
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func TestPlatformConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(PlatformConfigSuite))
}

type platformMock struct {
	noData bool

	hostname      []byte
	externalIPs   []netip.Addr
	addresses     []netip.Prefix
	defaultRoutes []netip.Addr
	linksUp       []string
	resolvers     []netip.Addr
	timeServers   []string
	dhcp4Links    []string
	tcpProbes     []string

	metadata *runtimeres.PlatformMetadataSpec
}

func (mock *platformMock) Name() string {
	return "mock"
}

func (mock *platformMock) Configuration(context.Context, state.State) ([]byte, error) {
	return nil, nil
}

func (mock *platformMock) Metadata(context.Context, state.State) (runtimeres.PlatformMetadataSpec, error) {
	return runtimeres.PlatformMetadataSpec{Platform: mock.Name()}, nil
}

func (mock *platformMock) Mode() v1alpha1runtime.Mode {
	return v1alpha1runtime.ModeCloud
}

func (mock *platformMock) KernelArgs(string, quirks.Quirks) procfs.Parameters {
	return nil
}

//nolint:gocyclo
func (mock *platformMock) NetworkConfiguration(
	ctx context.Context,
	st state.State,
	ch chan<- *v1alpha1runtime.PlatformNetworkConfig,
) error {
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
		if addr.Addr().Is6() {
			family = nethelpers.FamilyInet6
		}

		networkConfig.Addresses = append(
			networkConfig.Addresses,
			network.AddressSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				LinkName:    "eth0",
				Address:     addr,
				Scope:       nethelpers.ScopeGlobal,
				Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
				Family:      family,
			},
		)
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
		networkConfig.Links = append(
			networkConfig.Links, network.LinkSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				Name:        link,
				Up:          true,
			},
		)
	}

	if len(mock.resolvers) > 0 {
		networkConfig.Resolvers = append(
			networkConfig.Resolvers, network.ResolverSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				DNSServers:  mock.resolvers,
			},
		)
	}

	if len(mock.timeServers) > 0 {
		networkConfig.TimeServers = append(
			networkConfig.TimeServers, network.TimeServerSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				NTPServers:  mock.timeServers,
			},
		)
	}

	for _, link := range mock.dhcp4Links {
		networkConfig.Operators = append(
			networkConfig.Operators, network.OperatorSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				LinkName:    link,
				Operator:    network.OperatorDHCP4,
				DHCP4:       network.DHCP4OperatorSpec{},
			},
		)
	}

	for _, endpoint := range mock.tcpProbes {
		networkConfig.Probes = append(
			networkConfig.Probes, network.ProbeSpecSpec{
				Interval: time.Second,
				TCP: network.TCPProbeSpec{
					Endpoint: endpoint,
					Timeout:  time.Second,
				},
				ConfigLayer: network.ConfigPlatform,
			})
	}

	networkConfig.Metadata = mock.metadata

	for range 5 { // send the network config multiple times to test duplicate suppression
		select {
		case ch <- networkConfig:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}
