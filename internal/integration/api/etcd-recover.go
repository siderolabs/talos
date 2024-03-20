// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// EtcdRecoverSuite ...
type EtcdRecoverSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *EtcdRecoverSuite) SuiteName() string {
	return "api.EtcdRecoverSuite"
}

// SetupTest ...
func (suite *EtcdRecoverSuite) SetupTest() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	// make sure we abort at some point in time, but give enough room for Recovers
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 10*time.Minute)
}

// TearDownTest ...
func (suite *EtcdRecoverSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestSnapshotRecover snapshot etcd, wipes control plane nodes and recovers etcd from a snapshot.
//
//nolint:gocyclo
func (suite *EtcdRecoverSuite) TestSnapshotRecover() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot")
	}

	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reset test is not reliable (can't wait for cluster readiness in between resets)")
	}

	// 'init' nodes are not compatible with etcd recovery
	suite.Require().Empty(suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeInit))

	controlPlaneNodes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)
	suite.Require().NotEmpty(controlPlaneNodes)

	snapshotNode := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)
	recoverNode := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	suite.WaitForBootDone(suite.ctx)

	suite.T().Logf("taking etcd snapshot at node %q", snapshotNode)

	var snapshot bytes.Buffer

	suite.Require().NoError(suite.snapshotEtcd(snapshotNode, &snapshot))

	// wipe ephemeral partition on all control plane nodes
	preReset := map[string]string{}

	for _, node := range controlPlaneNodes {
		var err error

		preReset[node], err = suite.HashKubeletCert(suite.ctx, node)

		suite.Require().NoError(err)
	}

	suite.T().Logf("wiping control plane nodes %q", controlPlaneNodes)

	errCh := make(chan error)

	for _, node := range controlPlaneNodes {
		go func() {
			errCh <- func() error {
				nodeCtx := client.WithNodes(suite.ctx, node)

				bootIDBefore, err := suite.ReadBootID(nodeCtx)
				if err != nil {
					return fmt.Errorf("error reading pre-reset boot ID: %w", err)
				}

				if err = base.IgnoreGRPCUnavailable(
					suite.Client.ResetGeneric(
						nodeCtx, &machineapi.ResetRequest{
							Reboot:   true,
							Graceful: false,
							SystemPartitionsToWipe: []*machineapi.ResetPartitionSpec{
								{
									Label: constants.EphemeralPartitionLabel,
									Wipe:  true,
								},
							},
						},
					),
				); err != nil {
					return fmt.Errorf("error resetting the node %q: %w", node, err)
				}

				var bootIDAfter string

				return retry.Constant(5 * time.Minute).Retry(
					func() error {
						requestCtx, requestCtxCancel := context.WithTimeout(nodeCtx, 5*time.Second)
						defer requestCtxCancel()

						bootIDAfter, err = suite.ReadBootID(requestCtx)
						if err != nil {
							// API might be unresponsive during reboot
							return retry.ExpectedError(err)
						}

						if bootIDAfter == bootIDBefore {
							// bootID should be different after reboot
							return retry.ExpectedErrorf(
								"bootID didn't change for node %q: before %s, after %s",
								node,
								bootIDBefore,
								bootIDAfter,
							)
						}

						return nil
					},
				)
			}()
		}()
	}

	for range controlPlaneNodes {
		suite.Require().NoError(<-errCh)
	}

	suite.ClearConnectionRefused(suite.ctx, controlPlaneNodes...)

	suite.T().Logf("recovering etcd snapshot at node %q", recoverNode)

	suite.Require().NoError(suite.recoverEtcd(recoverNode, bytes.NewReader(snapshot.Bytes())))

	suite.AssertClusterHealthy(suite.ctx)

	for _, node := range controlPlaneNodes {
		postReset, err := suite.HashKubeletCert(suite.ctx, node)
		suite.Require().NoError(err)

		suite.Assert().NotEqual(postReset, preReset[node], "kubelet cert hasn't changed for node %q", node)
	}
}

func (suite *EtcdRecoverSuite) snapshotEtcd(snapshotNode string, dest io.Writer) error {
	ctx := client.WithNodes(suite.ctx, snapshotNode)

	r, err := suite.Client.EtcdSnapshot(ctx, &machineapi.EtcdSnapshotRequest{})
	if err != nil {
		return fmt.Errorf("error reading snapshot: %w", err)
	}

	defer r.Close() //nolint:errcheck

	_, err = io.Copy(dest, r)

	return err
}

func (suite *EtcdRecoverSuite) recoverEtcd(recoverNode string, src io.ReadSeeker) error {
	ctx := client.WithNodes(suite.ctx, recoverNode)

	suite.T().Log("uploading the snapshot")

	if err := retry.Constant(time.Minute, retry.WithUnits(time.Millisecond*200)).RetryWithContext(
		ctx, func(ctx context.Context) error {
			_, err := src.Seek(0, io.SeekStart)
			if err != nil {
				return err
			}

			_, err = suite.Client.EtcdRecover(ctx, src)

			if client.StatusCode(err) == codes.FailedPrecondition {
				return retry.ExpectedError(err)
			}

			return err
		},
	); err != nil {
		return fmt.Errorf("error uploading snapshot: %w", err)
	}

	suite.T().Log("bootstrapping from the snapshot")

	return retry.Constant(time.Minute, retry.WithUnits(time.Millisecond*200)).RetryWithContext(
		ctx, func(ctx context.Context) error {
			err := suite.Client.Bootstrap(
				ctx, &machineapi.BootstrapRequest{
					RecoverEtcd: true,
				},
			)

			if client.StatusCode(err) == codes.FailedPrecondition || client.StatusCode(err) == codes.DeadlineExceeded {
				return retry.ExpectedError(err)
			}

			return err
		},
	)
}

func init() {
	allSuites = append(allSuites, new(EtcdRecoverSuite))
}
