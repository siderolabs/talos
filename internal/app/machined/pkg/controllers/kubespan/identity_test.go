// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package kubespan_test

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	kubespanctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

type IdentitySuite struct {
	KubeSpanSuite

	statePath string
}

func (suite *IdentitySuite) TestGenerate() {
	suite.statePath = suite.T().TempDir()

	suite.Require().NoError(suite.runtime.RegisterController(&kubespanctrl.IdentityController{
		StatePath: suite.statePath,
	}))

	suite.startRuntime()

	stateMount := runtimeres.NewMountStatus(v1alpha1.NamespaceName, constants.StatePartitionLabel)

	suite.Assert().NoError(suite.state.Create(suite.ctx, stateMount))

	cfg := kubespan.NewConfig(config.NamespaceName, kubespan.ConfigID)
	cfg.TypedSpec().Enabled = true
	cfg.TypedSpec().ClusterID = "8XuV9TZHW08DOk3bVxQjH9ih_TBKjnh-j44tsCLSBzo="

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	firstMac := network.NewHardwareAddr(network.NamespaceName, network.FirstHardwareAddr)
	mac, err := net.ParseMAC("ea:71:1b:b2:cc:ee")
	suite.Require().NoError(err)

	firstMac.TypedSpec().HardwareAddr = nethelpers.HardwareAddr(mac)

	suite.Require().NoError(suite.state.Create(suite.ctx, firstMac))

	specMD := resource.NewMetadata(kubespan.NamespaceName, kubespan.IdentityType, kubespan.LocalIdentity, resource.VersionUndefined)

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(
			specMD,
			func(res resource.Resource) error {
				spec := res.(*kubespan.Identity).TypedSpec()

				_, err := wgtypes.ParseKey(spec.PrivateKey)
				suite.Assert().NoError(err)

				_, err = wgtypes.ParseKey(spec.PublicKey)
				suite.Assert().NoError(err)

				suite.Assert().Equal("fd7f:175a:b97c:5602:e871:1bff:feb2:ccee/128", spec.Address.String())
				suite.Assert().Equal("fd7f:175a:b97c:5602::/64", spec.Subnet.String())

				return nil
			},
		),
	))
}

func (suite *IdentitySuite) TestLoad() {
	// using verbatim data here to make sure nodeId representation is supported in future version of Talos
	const identityYaml = `address: ""
subnet: ""
privateKey: sF45u5ePau58WeeCUY3T8D9foEKaQ8Opx4cGC8g4XE4=
publicKey: Oak2fBEWngBhwslBxDVgnRNHXs88OAp4kjroSX0uqUE=
`

	suite.statePath = suite.T().TempDir()

	suite.Require().NoError(suite.runtime.RegisterController(&kubespanctrl.IdentityController{
		StatePath: suite.statePath,
	}))

	suite.startRuntime()

	suite.Require().NoError(os.WriteFile(filepath.Join(suite.statePath, constants.KubeSpanIdentityFilename), []byte(identityYaml), 0o600))

	stateMount := runtimeres.NewMountStatus(v1alpha1.NamespaceName, constants.StatePartitionLabel)

	suite.Assert().NoError(suite.state.Create(suite.ctx, stateMount))

	cfg := kubespan.NewConfig(config.NamespaceName, kubespan.ConfigID)
	cfg.TypedSpec().Enabled = true
	cfg.TypedSpec().ClusterID = "8XuV9TZHW08DOk3bVxQjH9ih_TBKjnh-j44tsCLSBzo="

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	firstMac := network.NewHardwareAddr(network.NamespaceName, network.FirstHardwareAddr)
	mac, err := net.ParseMAC("ea:71:1b:b2:cc:ee")
	suite.Require().NoError(err)

	firstMac.TypedSpec().HardwareAddr = nethelpers.HardwareAddr(mac)

	suite.Require().NoError(suite.state.Create(suite.ctx, firstMac))

	specMD := resource.NewMetadata(kubespan.NamespaceName, kubespan.IdentityType, kubespan.LocalIdentity, resource.VersionUndefined)

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(
			specMD,
			func(res resource.Resource) error {
				spec := res.(*kubespan.Identity).TypedSpec()

				suite.Assert().Equal("sF45u5ePau58WeeCUY3T8D9foEKaQ8Opx4cGC8g4XE4=", spec.PrivateKey)
				suite.Assert().Equal("Oak2fBEWngBhwslBxDVgnRNHXs88OAp4kjroSX0uqUE=", spec.PublicKey)
				suite.Assert().Equal("fd7f:175a:b97c:5602:e871:1bff:feb2:ccee/128", spec.Address.String())
				suite.Assert().Equal("fd7f:175a:b97c:5602::/64", spec.Subnet.String())

				return nil
			},
		),
	))
}

func TestIdentitySuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(IdentitySuite))
}
