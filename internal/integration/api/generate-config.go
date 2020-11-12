// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package api

import (
	"context"
	"fmt"
	"time"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/images"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// GenerateConfigSuite ...
type GenerateConfigSuite struct {
	base.K8sSuite

	ctx       context.Context
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *GenerateConfigSuite) SuiteName() string {
	return "api.GenerateConfigSuite"
}

// SetupTest ...
func (suite *GenerateConfigSuite) SetupTest() {
	// make sure we abort at some point in time, but give enough room for Recovers
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *GenerateConfigSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestGenerate verifies the generate config API.
func (suite *GenerateConfigSuite) TestGenerate() {
	request := &machineapi.GenerateConfigurationRequest{
		ConfigVersion: "v1alpha1",
		MachineConfig: &machineapi.MachineConfig{
			Type: machineapi.MachineConfig_MachineType(machine.TypeInit),
			NetworkConfig: &machineapi.NetworkConfig{
				Hostname: "testhost",
			},
			InstallConfig: &machineapi.InstallConfig{
				InstallDisk:  "/dev/sdb",
				InstallImage: images.DefaultInstallerImage,
			},
			KubernetesVersion: constants.DefaultKubernetesVersion,
		},
		ClusterConfig: &machineapi.ClusterConfig{
			Name: "talos-default",
			ControlPlane: &machineapi.ControlPlaneConfig{
				Endpoint: "http://localhost",
			},
			ClusterNetwork: &machineapi.ClusterNetworkConfig{
				DnsDomain: "cluster.test",
			},
		},
	}

	reply, err := suite.Client.GenerateConfiguration(
		suite.ctx,
		request,
	)

	suite.Require().NoError(err)

	config, err := configloader.NewFromBytes(reply.GetData()[0])

	suite.Require().NoError(err)

	suite.Require().EqualValues(config.Machine().Type(), request.MachineConfig.Type)
	suite.Require().EqualValues(config.Version(), request.ConfigVersion)
	suite.Require().EqualValues(config.Cluster().Name(), request.ClusterConfig.Name)
	suite.Require().EqualValues(config.Cluster().Endpoint(), request.ClusterConfig.ControlPlane.Endpoint)
	suite.Require().EqualValues(config.Cluster().Network().DNSDomain(), request.ClusterConfig.ClusterNetwork.DnsDomain)
	suite.Require().EqualValues(config.Machine().Kubelet().Image(), fmt.Sprintf("%s:%s", constants.KubeletImage, request.MachineConfig.KubernetesVersion))
	suite.Require().EqualValues(config.Machine().Install().Disk(), request.MachineConfig.InstallConfig.InstallDisk)
	suite.Require().EqualValues(config.Machine().Install().Image(), request.MachineConfig.InstallConfig.InstallImage)
	suite.Require().EqualValues(config.Machine().Network().Hostname(), request.MachineConfig.NetworkConfig.Hostname)
}
