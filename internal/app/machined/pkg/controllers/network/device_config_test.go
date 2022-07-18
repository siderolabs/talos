// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

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

	netctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

type DeviceConfigSpecSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *DeviceConfigSpecSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.DeviceConfigController{}))

	suite.startRuntime()
}

func (suite *DeviceConfigSpecSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *DeviceConfigSpecSuite) TestDeviceConfigs() {
	cfgProvider := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "eth0",
						DeviceAddresses: []string{"192.168.2.0/24"},
						DeviceMTU:       1500,
					},
				},
			},
		},
	}

	cfg := config.NewMachineConfig(cfgProvider)

	devices := map[string]*v1alpha1.Device{}
	for index, item := range cfgProvider.MachineConfig.MachineNetwork.NetworkInterfaces {
		devices[fmt.Sprintf("%s/%d", item.DeviceInterface, index)] = item
	}

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				list, err := suite.state.List(
					suite.ctx,
					resource.NewMetadata(network.NamespaceName, network.DeviceConfigSpecType, "", resource.VersionUndefined),
				)
				if err != nil {
					return err
				}

				for _, device := range list.Items {
					if _, ok := devices[device.Metadata().ID()]; !ok {
						return retry.ExpectedErrorf("device with id '%s' wasn't found", device.Metadata().ID())
					}

					delete(devices, device.Metadata().ID())
				}

				if len(list.Items) == 0 {
					return retry.ExpectedErrorf("no device configs were created yet")
				}

				return nil
			},
		))

	suite.Assert().Len(devices, 0)
}

func (suite *DeviceConfigSpecSuite) TestSelectors() {
	kernelDriver := "thedriver"

	cfgProvider := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceSelector: &v1alpha1.NetworkDeviceSelector{
							NetworkDeviceKernelDriver: kernelDriver,
						},
						DeviceAddresses: []string{"192.168.2.0/24"},
						DeviceMTU:       1500,
					},
				},
			},
		},
	}

	cfg := config.NewMachineConfig(cfgProvider)

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	status := network.NewLinkStatus(network.NamespaceName, "eth0")
	status.TypedSpec().Driver = kernelDriver

	suite.Require().NoError(suite.state.Create(suite.ctx, status))

	status = network.NewLinkStatus(network.NamespaceName, "eth1")
	suite.Require().NoError(suite.state.Create(suite.ctx, status))

	var deviceConfig *network.DeviceConfigSpec

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				config, err := suite.state.Get(
					suite.ctx,
					resource.NewMetadata(network.NamespaceName, network.DeviceConfigSpecType, "eth0/0", resource.VersionUndefined),
				)
				if err != nil {
					return retry.ExpectedError(err)
				}

				deviceConfig = config.(*network.DeviceConfigSpec) //nolint:errcheck,forcetypeassert

				return nil
			},
		))

	suite.Assert().NotNil(deviceConfig)
	suite.Assert().EqualValues(1500, deviceConfig.TypedSpec().Device.MTU())
}

func (suite *DeviceConfigSpecSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestDeviceConfigSpecSuite(t *testing.T) {
	suite.Run(t, new(DeviceConfigSpecSuite))
}
