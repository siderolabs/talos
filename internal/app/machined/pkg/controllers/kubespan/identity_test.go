// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package kubespan_test

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	kubespanctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type IdentitySuite struct {
	ctest.DefaultSuite
}

func (suite *IdentitySuite) TestGenerate() {
	cfg := kubespan.NewConfig(config.NamespaceName, kubespan.ConfigID)
	cfg.TypedSpec().Enabled = true
	cfg.TypedSpec().ClusterID = "8XuV9TZHW08DOk3bVxQjH9ih_TBKjnh-j44tsCLSBzo="

	suite.Create(cfg)

	firstMac := network.NewHardwareAddr(network.NamespaceName, network.FirstHardwareAddr)
	mac, err := net.ParseMAC("ea:71:1b:b2:cc:ee")
	suite.Require().NoError(err)

	firstMac.TypedSpec().HardwareAddr = nethelpers.HardwareAddr(mac)
	suite.Create(firstMac)

	statePath := suite.T().TempDir()
	mountID := (&kubespanctrl.IdentityController{}).Name() + "-" + constants.StatePartitionLabel

	ctest.AssertResource(suite, mountID, func(mountRequest *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(constants.StatePartitionLabel, mountRequest.TypedSpec().VolumeID)
	})

	ctest.AssertNoResource[*kubespan.Identity](suite, kubespan.LocalIdentity)

	volumeMountStatus := block.NewVolumeMountStatus(block.NamespaceName, mountID)
	volumeMountStatus.TypedSpec().Target = statePath
	suite.Create(volumeMountStatus)

	ctest.AssertResource(suite, kubespan.LocalIdentity, func(identity *kubespan.Identity, asrt *assert.Assertions) {
		spec := identity.TypedSpec()

		_, err := wgtypes.ParseKey(spec.PrivateKey)
		asrt.NoError(err)

		_, err = wgtypes.ParseKey(spec.PublicKey)
		asrt.NoError(err)

		asrt.Equal("fd7f:175a:b97c:5602:e871:1bff:feb2:ccee/128", spec.Address.String())
		asrt.Equal("fd7f:175a:b97c:5602::/64", spec.Subnet.String())
	})

	ctest.AssertResources(suite, []resource.ID{volumeMountStatus.Metadata().ID()}, func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(vms.Metadata().Finalizers().Empty())
	})

	suite.Destroy(volumeMountStatus)

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, mountID)
}

func (suite *IdentitySuite) TestLoad() {
	statePath := suite.T().TempDir()
	mountID := (&kubespanctrl.IdentityController{}).Name() + "-" + constants.StatePartitionLabel

	// using verbatim data here to make sure nodeId representation is supported in future version of Talos
	const identityYaml = `address: ""
subnet: ""
privateKey: sF45u5ePau58WeeCUY3T8D9foEKaQ8Opx4cGC8g4XE4=
publicKey: Oak2fBEWngBhwslBxDVgnRNHXs88OAp4kjroSX0uqUE=
`

	suite.Require().NoError(os.WriteFile(filepath.Join(statePath, constants.KubeSpanIdentityFilename), []byte(identityYaml), 0o600))

	cfg := kubespan.NewConfig(config.NamespaceName, kubespan.ConfigID)
	cfg.TypedSpec().Enabled = true
	cfg.TypedSpec().ClusterID = "8XuV9TZHW08DOk3bVxQjH9ih_TBKjnh-j44tsCLSBzo="

	suite.Create(cfg)

	firstMac := network.NewHardwareAddr(network.NamespaceName, network.FirstHardwareAddr)
	mac, err := net.ParseMAC("ea:71:1b:b2:cc:ee")
	suite.Require().NoError(err)

	firstMac.TypedSpec().HardwareAddr = nethelpers.HardwareAddr(mac)
	suite.Create(firstMac)

	ctest.AssertResource(suite, mountID, func(mountRequest *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(constants.StatePartitionLabel, mountRequest.TypedSpec().VolumeID)
	})

	ctest.AssertNoResource[*kubespan.Identity](suite, kubespan.LocalIdentity)

	volumeMountStatus := block.NewVolumeMountStatus(block.NamespaceName, mountID)
	volumeMountStatus.TypedSpec().Target = statePath
	suite.Create(volumeMountStatus)

	ctest.AssertResource(suite, kubespan.LocalIdentity, func(identity *kubespan.Identity, asrt *assert.Assertions) {
		spec := identity.TypedSpec()

		asrt.Equal("sF45u5ePau58WeeCUY3T8D9foEKaQ8Opx4cGC8g4XE4=", spec.PrivateKey)
		asrt.Equal("Oak2fBEWngBhwslBxDVgnRNHXs88OAp4kjroSX0uqUE=", spec.PublicKey)
		asrt.Equal("fd7f:175a:b97c:5602:e871:1bff:feb2:ccee/128", spec.Address.String())
		asrt.Equal("fd7f:175a:b97c:5602::/64", spec.Subnet.String())
	})

	ctest.AssertResources(suite, []resource.ID{volumeMountStatus.Metadata().ID()}, func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(vms.Metadata().Finalizers().Empty())
	})

	suite.Destroy(volumeMountStatus)

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, mountID)
}

func TestIdentitySuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &IdentitySuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&kubespanctrl.IdentityController{}))
			},
		},
	})
}
