// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_provision

package provision

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v4"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// MaintenanceSideroLinkSuite ...
type MaintenanceSideroLinkSuite struct {
	BaseSuite

	track int
}

// SuiteName ...
func (suite *MaintenanceSideroLinkSuite) SuiteName() string {
	return fmt.Sprintf("provision.UpgradeSuite.MaintenanceSideroLink-TR%d", suite.track)
}

// TestAPI tests basic maintenance API operations with SideroLink support.
//
//nolint:gocyclo,cyclop
func (suite *MaintenanceSideroLinkSuite) TestAPI() {
	const (
		maintenanceControlplanes = 1
		maintenanceWorkers       = 1
	)

	sourceInstallerImage := fmt.Sprintf(
		"%s/%s:%s",
		DefaultSettings.TargetInstallImageRegistry,
		images.DefaultInstallerImageName, //nolint:staticcheck // legacy is only used in tests
		DefaultSettings.CurrentVersion,
	)

	suite.setupCluster(clusterOptions{
		ClusterName: "maintenance-siderolink",

		ControlplaneNodes: maintenanceControlplanes,
		WorkerNodes:       maintenanceWorkers,

		SourceKernelPath:     helpers.ArtifactPath(constants.KernelAssetWithArch),
		SourceInitramfsPath:  helpers.ArtifactPath(constants.InitramfsAssetWithArch),
		SourceInstallerImage: sourceInstallerImage,
		SourceVersion:        DefaultSettings.CurrentVersion,
		SourceK8sVersion:     constants.DefaultKubernetesVersion,

		WithSkipInjectingConfig: true,
		WithSideroLink:          true,
	})

	maintenanceClients := make([]*client.Client, len(suite.Cluster.Info().Nodes))

	for i, machine := range suite.Cluster.Info().Nodes {
		var err error

		maintenanceClients[i], err = client.New(
			suite.ctx,
			client.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
			client.WithEndpoints(machine.IPs[0].String()),
		)
		suite.Require().NoError(err)
	}

	defer func() {
		for _, c := range maintenanceClients {
			suite.Require().NoError(c.Close())
		}
	}()

	suite.Run("wait for maintenance API", func() {
		// we should be able to query version API for every machine
		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			for _, maintenanceClient := range maintenanceClients {
				version, err := maintenanceClient.Version(suite.ctx)
				if !asrt.NoError(err) {
					return
				}

				suite.Assert().Equal(DefaultSettings.CurrentVersion, version.GetMessages()[0].GetVersion().GetTag())
			}
		}, time.Minute, time.Second, "version API should be available")
	})

	sideroLinkConfig := suite.slb.ConfigDocument(false)

	registryMirrorConfig, err := container.New(
		xslices.Filter(suite.configBundle.WorkerCfg.Documents(), func(doc config.Document) bool {
			return doc.Kind() == cri.RegistryMirrorConfig
		})...,
	)
	suite.Require().NoError(err)

	maintenancePatched, err := configpatcher.Apply(
		configpatcher.WithConfig(sideroLinkConfig),
		[]configpatcher.Patch{configpatcher.NewStrategicMergePatch(registryMirrorConfig)},
	)
	suite.Require().NoError(err)

	maintenanceConfig, err := maintenancePatched.Bytes()
	suite.Require().NoError(err)

	suite.Run("apply SideroLink configuration", func() {
		for _, maintenanceClient := range maintenanceClients {
			_, err := maintenanceClient.ApplyConfiguration(suite.ctx, &machine.ApplyConfigurationRequest{
				Data: maintenanceConfig,
			})
			suite.Require().NoError(err)
		}
	})

	suite.Run("wait for machines to stop listening on regular IPs", func() {
		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			for _, maintenanceClient := range maintenanceClients {
				_, err := maintenanceClient.Version(suite.ctx)
				if !asrt.Error(err) {
					return
				}

				asrt.ErrorContains(err, "connection refused")
			}
		}, time.Minute, time.Second, "machines should stop listening on regular IPs")
	})

	sideroLinkMaintenanceClients := make([]*client.Client, len(suite.Cluster.Info().Nodes))

	for i, sideroLinkIP := range slices.Concat(suite.controlplaneSideroLinkIPs, suite.workerSideroLinkIPs) {
		var err error

		sideroLinkMaintenanceClients[i], err = client.New(
			suite.ctx,
			client.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
			client.WithEndpoints(sideroLinkIP.String()),
		)
		suite.Require().NoError(err)
	}

	defer func() {
		for _, c := range sideroLinkMaintenanceClients {
			suite.Require().NoError(c.Close())
		}
	}()

	suite.Run("wait for machines to listen on SideroLink IPs", func() {
		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			for _, sideroLinkMaintenanceClient := range sideroLinkMaintenanceClients {
				version, err := sideroLinkMaintenanceClient.Version(suite.ctx)
				if !asrt.NoError(err) {
					return
				}

				suite.Assert().Equal(DefaultSettings.CurrentVersion, version.GetMessages()[0].GetVersion().GetTag())
			}
		}, time.Minute, time.Second, "machines should listen on SideroLink IPs")
	})

	suite.Run("wipe the disk", func() {
		// we can wipe the disk, as SideroLink client has admin permissions
		for _, maintenanceClient := range sideroLinkMaintenanceClients {
			suite.Require().NoError(maintenanceClient.BlockDeviceWipe(suite.ctx, &storage.BlockDeviceWipeRequest{
				Devices: []*storage.BlockDeviceWipeDescriptor{
					{
						Device: "vda",
						Method: storage.BlockDeviceWipeDescriptor_FAST,
					},
				},
			}))
		}
	})

	suite.Run("pull the installer image and install", func() {
		// we can pull the installer image, as SideroLink client has admin permissions
		for _, maintenanceClient := range sideroLinkMaintenanceClients {
			pullClient, err := maintenanceClient.ImageClient.Pull(suite.ctx, &machine.ImageServicePullRequest{
				Containerd: &common.ContainerdInstance{
					Driver:    common.ContainerDriver_CONTAINERD,
					Namespace: common.ContainerdNamespace_NS_SYSTEM,
				},
				ImageRef: sourceInstallerImage,
			})
			suite.Require().NoError(err)
			suite.Require().NoError(pullClient.CloseSend())

			var installImageName string

			for {
				msg, err := pullClient.Recv()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}

					suite.Require().NoError(err)
				}

				if name := msg.GetName(); name != "" {
					installImageName = name
				}
			}

			suite.Require().NotEmpty(installImageName, "installer image name should be returned")

			// try upgrade first, it should fail, as the machine hasn't been installed yet
			upgradeCli, err := maintenanceClient.LifecycleClient.Upgrade(suite.ctx, &machine.LifecycleServiceUpgradeRequest{
				Containerd: &common.ContainerdInstance{
					Driver:    common.ContainerDriver_CONTAINERD,
					Namespace: common.ContainerdNamespace_NS_SYSTEM,
				},
				Source: &machine.InstallArtifactsSource{
					ImageName: installImageName,
				},
			})
			suite.Require().NoError(err)
			suite.Require().NoError(upgradeCli.CloseSend())

			_, err = upgradeCli.Recv()
			suite.Require().Error(err, "upgrade should fail if the machine is not installed")
			suite.Assert().Equal(codes.FailedPrecondition, client.StatusCode(err))

			// now install the machine
			installCli, err := maintenanceClient.LifecycleClient.Install(suite.ctx, &machine.LifecycleServiceInstallRequest{
				Containerd: &common.ContainerdInstance{
					Driver:    common.ContainerDriver_CONTAINERD,
					Namespace: common.ContainerdNamespace_NS_SYSTEM,
				},
				Source: &machine.InstallArtifactsSource{
					ImageName: installImageName,
				},
				Destination: &machine.InstallDestination{
					Disk: "/dev/vda",
				},
			})
			suite.Require().NoError(err)
			suite.Require().NoError(installCli.CloseSend())

			for {
				msg, err := installCli.Recv()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}

					suite.Require().NoError(err)
				}

				progress := msg.GetProgress()

				if msg := progress.GetMessage(); msg != "" {
					suite.T().Logf("install progress: %s", msg)
				}

				if exitCode := progress.GetExitCode(); exitCode != 0 {
					suite.Failf("install failed", "exit code %d", exitCode)
				}
			}
		}
	})

	suite.Run("wait for system disk to be established", func() {
		ctx, cancel := context.WithTimeout(suite.ctx, 15*time.Second)
		defer cancel()

		for _, maintenanceClient := range sideroLinkMaintenanceClients {
			rtestutils.AssertResource(
				ctx, suite.T(), maintenanceClient.COSI, block.SystemDiskID,
				func(systemDisk *block.SystemDisk, asrt *assert.Assertions) {
					asrt.Equal("/dev/vda", systemDisk.TypedSpec().DevPath)
				},
			)
		}
	})

	suite.Run("wait for META to be established", func() {
		ctx, cancel := context.WithTimeout(suite.ctx, 15*time.Second)
		defer cancel()

		for _, maintenanceClient := range sideroLinkMaintenanceClients {
			rtestutils.AssertResource(
				ctx, suite.T(), maintenanceClient.COSI, constants.MetaPartitionLabel,
				func(volumeStatus *block.VolumeStatus, asrt *assert.Assertions) {
					asrt.Equal(block.VolumePhaseReady, volumeStatus.TypedSpec().Phase)
				},
			)
		}
	})

	suite.Run("write META value", func() {
		maintenanceClient := sideroLinkMaintenanceClients[0]

		suite.Require().NoError(maintenanceClient.MetaWrite(suite.ctx, meta.UserReserved1, []byte("provision")))
	})

	suite.Run("reboot one machine", func() {
		maintenanceClient := sideroLinkMaintenanceClients[0]
		insecureMaintenanceClient := maintenanceClients[0]

		currentBootID := suite.readBootID(maintenanceClient)

		suite.Require().NoError(maintenanceClient.Reboot(suite.ctx))

		// after the reboot the machine should lose SideroLink config, so we expect it to come back up on regular IP
		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			version, err := insecureMaintenanceClient.Version(suite.ctx)
			if !asrt.NoError(err) {
				return
			}

			suite.Assert().Equal(DefaultSettings.CurrentVersion, version.GetMessages()[0].GetVersion().GetTag())
		}, time.Minute, time.Second, "version API should be available after reboot")

		// apply back SideroLink config
		_, err := insecureMaintenanceClient.ApplyConfiguration(suite.ctx, &machine.ApplyConfigurationRequest{
			Data: maintenanceConfig,
		})
		suite.Require().NoError(err)

		// wait for the machine to come back on SideroLink IP
		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			version, err := maintenanceClient.Version(suite.ctx)
			if !asrt.NoError(err) {
				return
			}

			suite.Assert().Equal(DefaultSettings.CurrentVersion, version.GetMessages()[0].GetVersion().GetTag())
		}, time.Minute, time.Second, "API should listen on SideroLink IP after reboot")

		// check that boot ID has changed after reboot
		newBootID := suite.readBootID(maintenanceClient)
		suite.Assert().NotEqual(currentBootID, newBootID, "boot ID should change after reboot")
	})

	suite.Run("read back META value", func() {
		ctx, cancel := context.WithTimeout(suite.ctx, 15*time.Second)
		defer cancel()

		maintenanceClient := sideroLinkMaintenanceClients[0]

		rtestutils.AssertResource(
			ctx, suite.T(), maintenanceClient.COSI, runtime.MetaKeyTagToID(meta.UserReserved1),
			func(metaValue *runtime.MetaKey, asrt *assert.Assertions) {
				asrt.Equal("provision", metaValue.TypedSpec().Value)
			},
		)
	})

	suite.Run("upgrade using legacy API", func() {
		maintenanceClient := sideroLinkMaintenanceClients[0]
		insecureMaintenanceClient := maintenanceClients[0]

		currentBootID := suite.readBootID(maintenanceClient)

		_, err := maintenanceClient.Upgrade(suite.ctx, sourceInstallerImage, false, false) //nolint:staticcheck // legacy API test
		suite.Require().NoError(err)

		// after the reboot the machine should lose SideroLink config, so we expect it to come back up on regular IP
		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			version, err := insecureMaintenanceClient.Version(suite.ctx)
			if !asrt.NoError(err) {
				return
			}

			suite.Assert().Equal(DefaultSettings.CurrentVersion, version.GetMessages()[0].GetVersion().GetTag())
		}, time.Minute, time.Second, "version API should be available after reboot")

		// apply back SideroLink config
		_, err = insecureMaintenanceClient.ApplyConfiguration(suite.ctx, &machine.ApplyConfigurationRequest{
			Data: maintenanceConfig,
		})
		suite.Require().NoError(err)

		// wait for the machine to come back on SideroLink IP
		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			version, err := maintenanceClient.Version(suite.ctx)
			if !asrt.NoError(err) {
				return
			}

			suite.Assert().Equal(DefaultSettings.CurrentVersion, version.GetMessages()[0].GetVersion().GetTag())
		}, time.Minute, time.Second, "API should listen on SideroLink IP after reboot")

		// check that boot ID has changed after reboot
		newBootID := suite.readBootID(maintenanceClient)
		suite.Assert().NotEqual(currentBootID, newBootID, "boot ID should change after reboot")
	})

	suite.Run("apply config and have a cluster", func() {
		// drop .machine.install, as the machine is already installed
		patchRemoveInstall := map[string]any{
			"machine": map[string]any{
				"install": map[string]any{
					"$patch": "delete",
				},
			},
		}
		patchMarshaled, err := yaml.Marshal(patchRemoveInstall)
		suite.Require().NoError(err)

		patch, err := configpatcher.LoadPatch(patchMarshaled)
		suite.Require().NoError(err)

		for i := range maintenanceControlplanes {
			maintenanceClient := sideroLinkMaintenanceClients[i]

			patched, err := configpatcher.Apply(configpatcher.WithConfig(suite.configBundle.ControlPlaneCfg), []configpatcher.Patch{patch})
			suite.Require().NoError(err)

			configData, err := patched.Bytes()
			suite.Require().NoError(err)

			_, err = maintenanceClient.ApplyConfiguration(suite.ctx, &machine.ApplyConfigurationRequest{
				Data: configData,
			})
			suite.Require().NoError(err)
		}

		for i := range maintenanceWorkers {
			maintenanceClient := sideroLinkMaintenanceClients[maintenanceControlplanes+i]

			patched, err := configpatcher.Apply(configpatcher.WithConfig(suite.configBundle.WorkerCfg), []configpatcher.Patch{patch})
			suite.Require().NoError(err)

			configData, err := patched.Bytes()
			suite.Require().NoError(err)

			_, err = maintenanceClient.ApplyConfiguration(suite.ctx, &machine.ApplyConfigurationRequest{
				Data: configData,
			})
			suite.Require().NoError(err)
		}

		suite.Require().NoError(suite.clusterAccess.Bootstrap(suite.ctx, suite.T().Output()))

		suite.waitForClusterHealth()
	})
}

func (suite *MaintenanceSideroLinkSuite) readBootID(maintenanceClient *client.Client) string {
	reqCtx, reqCtxCancel := context.WithTimeout(suite.ctx, 10*time.Second)
	defer reqCtxCancel()

	reader, err := maintenanceClient.Read(reqCtx, "/proc/sys/kernel/random/boot_id")
	suite.Require().NoError(err)

	defer reader.Close() //nolint:errcheck

	body, err := io.ReadAll(reader)
	suite.Require().NoError(err)

	bootID := strings.TrimSpace(string(body))

	_, err = io.Copy(io.Discard, reader)
	suite.Require().NoError(err)

	suite.Require().NoError(reader.Close())

	return bootID
}

func init() {
	allSuites = append(
		allSuites,
		&MaintenanceSideroLinkSuite{track: 3},
	)
}
