// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_k8s

package k8s

import (
	"context"
	_ "embed"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// OomSuite verifies that userspace OOM handler will kill excessive replicas of a heavy memory consumer deployment.
type OomSuite struct {
	base.K8sSuite
}

var (
	//go:embed testdata/oom.yaml
	oomPodSpec []byte

	//go:embed testdata/oom-50-replicas.yaml
	oom50ReplicasPatch []byte

	//go:embed testdata/oom-1-replica.yaml
	oom1ReplicaPatch []byte
)

// SuiteName returns the name of the suite.
func (suite *OomSuite) SuiteName() string {
	return "k8s.OomSuite"
}

// TestOom verifies that system remains stable after handling an OOM event.
func (suite *OomSuite) TestOom() {
	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reaching out to the node IP is not reliable")
	}

	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping OOM test since provisioner is not qemu")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	suite.T().Cleanup(cancel)

	oomPodManifest := suite.ParseManifests(oomPodSpec)

	suite.T().Cleanup(func() {
		cleanUpCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()

		suite.DeleteManifests(cleanUpCtx, oomPodManifest)
	})

	suite.ApplyManifests(ctx, oomPodManifest)

	suite.Require().NoError(suite.WaitForDeploymentAvailable(ctx, time.Minute, "default", "stress-mem", 2))

	// Scale to 50
	suite.PatchK8sObject(ctx, "default", "apps", "Deployment", "v1", "stress-mem", oom50ReplicasPatch)

	// Expect at least one OOM kill of stress-ng within 15 seconds
	suite.Assert().True(suite.waitForOOMKilled(ctx, 15*time.Second, "stress-ng"))

	// Scale to 1, wait for deployment to scale down, proving system is operational
	suite.PatchK8sObject(ctx, "default", "apps", "Deployment", "v1", "stress-mem", oom1ReplicaPatch)
	suite.Require().NoError(suite.WaitForDeploymentAvailable(ctx, time.Minute, "default", "stress-mem", 1))

	suite.APISuite.AssertClusterHealthy(ctx)
}

// Waits for a period of time and return returns whether or not OOM events containing a specified process have been observed.
func (suite *OomSuite) waitForOOMKilled(ctx context.Context, timeout time.Duration, substr string) bool {
	startTime := time.Now()

	watchCh := make(chan state.Event)
	workerNode := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	workerCtx := client.WithNode(ctx, workerNode)

	suite.Assert().NoError(suite.Client.COSI.WatchKind(
		workerCtx,
		runtime.NewOOMActionSpec(runtime.NamespaceName, "").Metadata(),
		watchCh,
	))

	timeoutCh := time.After(timeout)
	ret := false

	for {
		select {
		case <-timeoutCh:
			return ret
		case ev := <-watchCh:
			if ev.Type != state.Created || ev.Resource.Metadata().Created().Before(startTime) {
				continue
			}

			res := ev.Resource.(*runtime.OOMAction).TypedSpec()

			for _, proc := range res.Processes {
				if strings.Contains(proc, substr) {
					ret = true
				}
			}
		}
	}
}

func init() {
	allSuites = append(allSuites, new(OomSuite))
}
