// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/internal/integration/base"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// EtcdRecoverSuite ...
type EtcdRecoverSuite struct {
	base.K8sSuite

	ctx       context.Context
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
//nolint: gocyclo
func (suite *EtcdRecoverSuite) TestSnapshotRecover() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot")
	}

	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reset test is not reliable (can't wait for cluster readiness in between resets)")
	}

	// 'init' nodes are not compatible with etcd recovery
	suite.Require().Empty(suite.DiscoverNodes().NodesByType(machine.TypeInit))

	controlPlaneNodes := suite.DiscoverNodes().NodesByType(machine.TypeControlPlane)
	suite.Require().NotEmpty(controlPlaneNodes)

	snapshotNode := suite.RandomDiscoveredNode(machine.TypeControlPlane)
	recoverNode := suite.RandomDiscoveredNode(machine.TypeControlPlane)

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
		node := node

		go func() {
			errCh <- func() error {
				nodeCtx := client.WithNodes(suite.ctx, node)

				bootIDBefore, err := suite.ReadBootID(nodeCtx)
				if err != nil {
					return fmt.Errorf("error reading pre-reset boot ID: %w", err)
				}

				if err = base.IgnoreGRPCUnavailable(suite.Client.ResetGeneric(nodeCtx, &machineapi.ResetRequest{
					Reboot:   true,
					Graceful: false,
					SystemPartitionsToWipe: []*machineapi.ResetPartitionSpec{
						{
							Label: constants.EphemeralPartitionLabel,
							Wipe:  true,
						},
					},
				})); err != nil {
					return fmt.Errorf("error resetting the node %q: %w", node, err)
				}

				var bootIDAfter string

				return retry.Constant(5 * time.Minute).Retry(func() error {
					requestCtx, requestCtxCancel := context.WithTimeout(nodeCtx, 5*time.Second)
					defer requestCtxCancel()

					bootIDAfter, err = suite.ReadBootID(requestCtx)

					if err != nil {
						// API might be unresponsive during reboot
						return retry.ExpectedError(err)
					}

					if bootIDAfter == bootIDBefore {
						// bootID should be different after reboot
						return retry.ExpectedError(fmt.Errorf("bootID didn't change for node %q: before %s, after %s", node, bootIDBefore, bootIDAfter))
					}

					return nil
				})
			}()
		}()
	}

	for range controlPlaneNodes {
		suite.Require().NoError(<-errCh)
	}

	suite.ClearConnectionRefused(suite.ctx, controlPlaneNodes...)

	suite.T().Logf("recovering etcd snapshot at node %q", recoverNode)

	suite.Require().NoError(suite.recoverEtcd(recoverNode, &snapshot))

	suite.AssertClusterHealthy(suite.ctx)

	for _, node := range controlPlaneNodes {
		postReset, err := suite.HashKubeletCert(suite.ctx, node)
		suite.Require().NoError(err)

		suite.Assert().NotEqual(postReset, preReset[node], "kubelet cert hasn't changed for node %q", node)
	}
}

func (suite *EtcdRecoverSuite) snapshotEtcd(snapshotNode string, dest io.Writer) error {
	ctx := client.WithNodes(suite.ctx, snapshotNode)

	r, errCh, err := suite.Client.EtcdSnapshot(ctx, &machineapi.EtcdSnapshotRequest{})
	if err != nil {
		return fmt.Errorf("error reading snapshot: %w", err)
	}

	defer r.Close() //nolint:errcheck

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		for err := range errCh {
			suite.T().Logf("read error: %s", err)
		}
	}()

	defer wg.Wait()

	_, err = io.Copy(dest, r)

	return err
}

func (suite *EtcdRecoverSuite) recoverEtcd(recoverNode string, src io.Reader) error {
	ctx := client.WithNodes(suite.ctx, recoverNode)

	_, err := suite.Client.EtcdRecover(ctx, src)
	if err != nil {
		return fmt.Errorf("error uploading snapshot: %w", err)
	}

	return suite.Client.Bootstrap(ctx, &machineapi.BootstrapRequest{
		RecoverEtcd: true,
	})
}

func init() {
	allSuites = append(allSuites, new(EtcdRecoverSuite))
}
