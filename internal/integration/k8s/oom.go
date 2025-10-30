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
	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// OomSuite verifies that userspace OOM handler will kill excessive replicas of a heavy memory consumer deployment.
type OomSuite struct {
	base.K8sSuite
}

//go:embed testdata/oom.yaml
var oomPodSpec []byte

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

	if suite.Race {
		suite.T().Skip("skipping as OOM tests are incompatible with race detector")
	}

	if suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping OOM test since provisioner is not qemu")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	suite.T().Cleanup(cancel)

	oomPodManifest := suite.ParseManifests(oomPodSpec)

	suite.T().Cleanup(func() {
		cleanUpCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		suite.DeleteManifests(cleanUpCtx, oomPodManifest)

		ticker := time.NewTicker(time.Second)
		done := cleanUpCtx.Done()

		// Wait for all stress-mem pods to complete terminating
		for {
			select {
			case <-ticker.C:
				pods, err := suite.Clientset.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
					LabelSelector: "app=stress-mem",
				})

				suite.Require().NoError(err)

				if len(pods.Items) == 0 {
					return
				}
			case <-done:
				suite.Require().Fail("Timed out waiting for cleanup")

				return
			}
		}
	})

	suite.ApplyManifests(ctx, oomPodManifest)

	suite.Require().NoError(suite.WaitForDeploymentAvailable(ctx, time.Minute, "default", "stress-mem", 2))

	// Figure out number of replicas, this is ballpark estimation of 15 replicas per 2GB of memory (per worker node)
	numWorkers := len(suite.DiscoverNodeInternalIPsByType(ctx, machine.TypeWorker))
	suite.Require().Greaterf(numWorkers, 0, "at least one worker node is required for the test")

	memInfo, err := suite.Client.Memory(client.WithNode(ctx, suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)))
	suite.Require().NoError(err)

	memoryBytes := memInfo.GetMessages()[0].GetMeminfo().GetMemtotal() * 1024
	numReplicas := int((memoryBytes/1024/1024+2048-1)/2048) * numWorkers * 25

	suite.T().Logf("detected memory: %s, workers %d => scaling to %d replicas",
		humanize.IBytes(memoryBytes), numWorkers, numReplicas)

	// Scale to discovered number of replicas
	suite.PatchK8sObject(ctx, "default", "apps", "Deployment", "v1", "stress-mem", patchToReplicas(suite.T(), numReplicas))

	// Expect at least one OOM kill of stress-ng within 15 seconds
	suite.Assert().True(suite.waitForOOMKilled(ctx, 15*time.Second, 2*time.Minute, "stress-ng", 1))

	// Scale to 1, wait for deployment to scale down, proving system is operational
	suite.PatchK8sObject(ctx, "default", "apps", "Deployment", "v1", "stress-mem", patchToReplicas(suite.T(), 1))
	suite.Require().NoError(suite.WaitForDeploymentAvailable(ctx, time.Minute, "default", "stress-mem", 1))

	// Monitor OOM kills for 15 seconds and make sure no kills other than stress-ng happen
	// Allow 0 as well: ideally that'd be the case, but fail on anything not containing stress-ng
	suite.Assert().True(suite.waitForOOMKilled(ctx, 15*time.Second, 2*time.Minute, "stress-ng", 0))

	suite.APISuite.AssertClusterHealthy(ctx)
}

func patchToReplicas(t *testing.T, replicas int) []byte {
	spec := map[string]any{
		"spec": map[string]any{
			"replicas": replicas,
		},
	}

	patch, err := yaml.Marshal(spec)
	require.NoError(t, err)

	return patch
}

// Waits for a period of time and return returns whether or not OOM events containing a specified process have been observed.
//
//nolint:gocyclo
func (suite *OomSuite) waitForOOMKilled(ctx context.Context, timeToObserve, timeout time.Duration, substr string, n int) bool {
	startTime := time.Now()

	watchCh := make(chan state.Event)
	workerNodes := suite.DiscoverNodeInternalIPsByType(ctx, machine.TypeWorker)

	// start watching OOM events on all worker nodes
	for _, workerNode := range workerNodes {
		suite.Assert().NoError(suite.Client.COSI.WatchKind(
			client.WithNode(ctx, workerNode),
			runtime.NewOOMActionSpec(runtime.NamespaceName, "").Metadata(),
			watchCh,
		))
	}

	timeoutCh := time.After(timeout)
	timeToObserveCh := time.After(timeToObserve)
	numOOMObserved := 0

	for {
		select {
		case <-timeoutCh:
			suite.T().Logf("observed %d OOM events containing process substring %q", numOOMObserved, substr)

			return numOOMObserved >= n
		case <-timeToObserveCh:
			if numOOMObserved >= n {
				// if we already observed some OOM events, consider it a success
				suite.T().Logf("observed %d OOM events containing process substring %q", numOOMObserved, substr)

				return true
			}
		case ev := <-watchCh:
			if ev.Type != state.Created || ev.Resource.Metadata().Created().Before(startTime) {
				continue
			}

			res := ev.Resource.(*runtime.OOMAction).TypedSpec()

			bailOut := false

			for _, proc := range res.Processes {
				if strings.Contains(proc, substr) {
					numOOMObserved++

					break
				}

				// Sometimes OOM catches containers in restart phase (while the
				// cgroup has previously accumulated OOM score).
				// Consider an OOM event wrong if something other than that is found.
				if !strings.Contains(proc, "runc init") && !strings.Contains(proc, "/pause") && proc != "" {
					bailOut = true
				}
			}

			if bailOut {
				suite.T().Logf("observed an OOM event not containing process substring %q: %q (%d containing)", substr, res.Processes, numOOMObserved)

				return false
			}
		}
	}
}

func init() {
	allSuites = append(allSuites, new(OomSuite))
}
