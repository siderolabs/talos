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
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/api"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// MaintenanceBasicSuite ...
type MaintenanceBasicSuite struct {
	BaseSuite

	track int
}

// SuiteName ...
func (suite *MaintenanceBasicSuite) SuiteName() string {
	return fmt.Sprintf("provision.UpgradeSuite.MaintenanceBasic-TR%d", suite.track)
}

// TestAPI tests basic maintenance API operations.
//
//nolint:gocyclo,cyclop
func (suite *MaintenanceBasicSuite) TestAPI() {
	const (
		maintenanceControlplanes = 1
		maintenanceWorkers       = 1
	)

	suite.setupCluster(clusterOptions{
		ClusterName: "maintenance",

		ControlplaneNodes: maintenanceControlplanes,
		WorkerNodes:       maintenanceWorkers,

		SourceKernelPath:    helpers.ArtifactPath(constants.KernelAssetWithArch),
		SourceInitramfsPath: helpers.ArtifactPath(constants.InitramfsAssetWithArch),
		SourceInstallerImage: fmt.Sprintf(
			"%s/%s:%s",
			DefaultSettings.TargetInstallImageRegistry,
			images.DefaultInstallerImageName,
			DefaultSettings.CurrentVersion,
		),
		SourceVersion:    DefaultSettings.CurrentVersion,
		SourceK8sVersion: constants.DefaultKubernetesVersion,

		WithSkipInjectingConfig: true,
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

	suite.Run("testing basic maintenance APIs", func() {
		// it doesn't matter which machine to use, as they are all same in maintenance mode right now
		maintenanceClient := maintenanceClients[0]

		linkStatuses, err := safe.ReaderListAll[*network.LinkStatus](suite.ctx, maintenanceClient.COSI)
		suite.Require().NoError(err)
		suite.Assert().NotEmpty(linkStatuses)

		// link specs should be not available (sensitive)
		_, err = safe.ReaderListAll[*network.LinkSpec](suite.ctx, maintenanceClient.COSI)
		suite.Require().Error(err)
		suite.Require().Equal(codes.PermissionDenied, client.StatusCode(err))

		// install API should not be allowed in maintenance mode
		installClient, err := maintenanceClient.LifecycleClient.Install(suite.ctx, &machine.LifecycleServiceInstallRequest{})
		suite.Require().NoError(err)

		_, err = installClient.Recv()
		suite.Require().Error(err)
		suite.Require().Equal(codes.PermissionDenied, client.StatusCode(err))

		// reboot should be not authorized in maintenance mode
		err = maintenanceClient.Reboot(suite.ctx)
		suite.Require().Error(err)
		suite.Require().Equal(codes.PermissionDenied, client.StatusCode(err))

		listClient, err := maintenanceClient.ImageClient.List(suite.ctx, &machine.ImageServiceListRequest{
			Containerd: &common.ContainerdInstance{
				Driver:    common.ContainerDriver_CONTAINERD,
				Namespace: common.ContainerdNamespace_NS_SYSTEM,
			},
		})
		suite.Require().NoError(err)

		for {
			_, err := listClient.Recv()
			if errors.Is(err, io.EOF) {
				break
			}

			suite.Require().NoError(err)
		}

		// block device wipe should be allowed in maintenance mode
		suite.Require().NoError(maintenanceClient.BlockDeviceWipe(suite.ctx, &storage.BlockDeviceWipeRequest{
			Devices: []*storage.BlockDeviceWipeDescriptor{
				{
					Device: "vda",
					Method: storage.BlockDeviceWipeDescriptor_FAST,
				},
			},
		}))
	})

	suite.Run("test all APIs in maintenance mode", func() {
		// it doesn't matter which machine to use, as they are all same in maintenance mode right now
		conn, err := grpc.NewClient(
			nethelpers.JoinHostPort(suite.Cluster.Info().Nodes[0].IPs[0].String(), constants.ApidPort),
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
		)
		suite.Require().NoError(err)

		defer func() {
			suite.Require().NoError(conn.Close())
		}()

		for _, service := range api.TalosAPIdAllAPIs() {
			for i := range service.Services().Len() {
				svc := service.Services().Get(i)

				for j := range svc.Methods().Len() {
					method := svc.Methods().Get(j)

					methodName := fmt.Sprintf("/%s/%s", svc.FullName(), method.Name())

					suite.Run(methodName, func() {
						// some APIs might be blocking
						ctx, cancel := context.WithTimeout(suite.ctx, time.Second)
						defer cancel()

						stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true, ClientStreams: true}, methodName)
						suite.Require().NoError(err)

						suite.Require().NoError(stream.SendMsg(&emptypb.Empty{}))
						suite.Require().NoError(stream.CloseSend())

						err = stream.RecvMsg(&emptypb.Empty{})

						switch {
						case status.Code(err) == codes.PermissionDenied:
							// expected, okay to have APIs that are not allowed in maintenance mode
						case status.Code(err) == codes.Unimplemented:
							// expected, some APIs might not be implemented in maintenance mode
						case status.Code(err) == codes.InvalidArgument:
							// expected, we submitted empty message, so some APIs might return invalid argument error
						case status.Code(err) == codes.DeadlineExceeded || status.Code(err) == codes.Canceled:
							// expected, API blocked, so deadline exceeded/canceled, also okay
						case err == nil:
							// also okay, some APIs might work in maintenance mode
							//
							// drain the stream
							for {
								err = stream.RecvMsg(&emptypb.Empty{})
								if errors.Is(err, io.EOF) {
									break
								}

								if status.Code(err) == codes.Canceled || status.Code(err) == codes.DeadlineExceeded {
									// expected, API blocked, so deadline exceeded/canceled, also okay
									break
								}

								suite.Assert().NoError(err, "unexpected error for method on draining the stream %s", methodName)
							}
						case errors.Is(err, io.EOF):
							// API done, also okay
						default:
							suite.Assert().NoError(err, "unexpected error for method %s", methodName)
						}

						suite.T().Logf("method %s -> %v", methodName, err)
					})
				}
			}
		}
	})

	suite.Run("apply config and have a cluster", func() {
		for i := range maintenanceControlplanes {
			maintenanceClient := maintenanceClients[i]

			configData, err := suite.configBundle.ControlPlaneCfg.Bytes()
			suite.Require().NoError(err)

			_, err = maintenanceClient.ApplyConfiguration(suite.ctx, &machine.ApplyConfigurationRequest{
				Data: configData,
			})
			suite.Require().NoError(err)
		}

		for i := range maintenanceWorkers {
			maintenanceClient := maintenanceClients[maintenanceControlplanes+i]

			configData, err := suite.configBundle.WorkerCfg.Bytes()
			suite.Require().NoError(err)

			_, err = maintenanceClient.ApplyConfiguration(suite.ctx, &machine.ApplyConfigurationRequest{
				Data: configData,
			})
			suite.Require().NoError(err)
		}

		suite.Require().NoError(suite.clusterAccess.Bootstrap(suite.ctx, suite.T().Output()))

		suite.waitForClusterHealth()
	})

	suite.Run("reset STATE and EPHEMERAL", func() {
		// reset starting from worker nodes
		for idx := len(suite.Cluster.Info().Nodes) - 1; idx >= 0; idx-- {
			node := suite.Cluster.Info().Nodes[idx].IPs[0].String()

			suite.Run(fmt.Sprintf("resetting node %s", node), func() {
				client, err := suite.clusterAccess.Client(node)
				suite.Require().NoError(err)

				defer func() {
					suite.Require().NoError(client.Close())
				}()

				suite.Require().NoError(client.ResetGeneric(suite.ctx, &machine.ResetRequest{
					Graceful: false,
					Reboot:   true,
					SystemPartitionsToWipe: []*machine.ResetPartitionSpec{
						{
							Label: constants.StatePartitionLabel,
							Wipe:  true,
						},
						{
							Label: constants.EphemeralPartitionLabel,
							Wipe:  true,
						},
					},
				}))
			})
		}
	})

	suite.Run("wait for back to maintenance API", func() {
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
		}, 3*time.Minute, time.Second, "version API should be available")
	})
}

func init() {
	allSuites = append(
		allSuites,
		&MaintenanceBasicSuite{track: 3},
	)
}
