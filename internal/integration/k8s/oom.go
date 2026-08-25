// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_k8s

package k8s

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"maps"
	"strconv"
	"strings"
	"sync"
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

	// overarching timeout should be longer than the sum of all timeouts in the test,
	// with enough slack for the cluster health check at the end
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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
				pods, err := suite.Clientset.CoreV1().Pods("default").List(cleanUpCtx, metav1.ListOptions{
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

	// Expect at least one OOM kill of stress-ng within 15 seconds, either by the Talos
	// userspace OOM handler, or by the kernel OOM killer
	suite.Assert().True(suite.waitForOOMKilled(ctx, 15*time.Second, 2*time.Minute, "stress-ng", 1, false))

	// Scale to 1, wait for deployment to scale down, proving system is operational
	suite.PatchK8sObject(ctx, "default", "apps", "Deployment", "v1", "stress-mem", patchToReplicas(suite.T(), 1))
	suite.Require().NoError(suite.WaitForDeploymentAvailable(ctx, time.Minute, "default", "stress-mem", 1))

	// Monitor OOM kills for 15 seconds and log any kills other than stress-ng.
	// Allow 0 as well: ideally that'd be the case, but allow other than stress-ng kills as well,
	// as OOM pressure doesn't go down immediately, and some other processes might get killed in the meantime.
	// The main point is to make sure that the system is stable via AssertClusterHealthy.
	suite.Assert().True(suite.waitForOOMKilled(ctx, 15*time.Second, 2*time.Minute, "stress-ng", 0, true))

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

// kernelOOMReadTimeout bounds a single read of the kernel OOM kill counter.
//
// A node under heavy memory pressure might be slow to respond or not respond at all, and the
// test should never block on it: the counters are a best-effort signal.
const kernelOOMReadTimeout = 5 * time.Second

// waitForOOMKilled waits for OOM kills to be observed on the worker nodes.
//
// Two independent sources are counted and reported separately:
//   - userspace OOM kills performed by the Talos OOM handler (OOMAction resources) which
//     contain the specified process substring;
//   - kernel OOM kills, as reported by the `oom_kill` counter in /proc/vmstat, summed over
//     all worker nodes (the kernel counter is not process-specific).
//
// It returns true if at least n kills from either source are observed within the observation
// period or before the timeout expires. If a non-matching userspace OOM kill is observed, it
// returns false immediately when allowNotMatchingKills is false; otherwise, such events are
// ignored.
//
//nolint:gocyclo
func (suite *OomSuite) waitForOOMKilled(ctx context.Context, timeToObserve, timeout time.Duration, substr string, n int, allowNotMatchingKills bool) bool {
	startTime := time.Now()

	watchCh := make(chan state.Event)
	workerNodes := suite.DiscoverNodeInternalIPsByType(ctx, machine.TypeWorker)

	// reads of the kernel counters should outlive the watch context below, as the last read
	// happens once the observation window is over
	readCtx := ctx

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// baseline for the kernel OOM kill counters, so that only the kills happening from now on are counted
	kernelOOM := suite.newKernelOOMTracker(readCtx, workerNodes)

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

	// the kernel counters are not exposed as an event stream, so they have to be polled
	kernelPollTicker := time.NewTicker(time.Second)
	defer kernelPollTicker.Stop()

	numOOMObserved, numKernelOOMObserved := 0, 0

	report := func() {
		suite.T().Logf("observed %d userspace OOM events containing process substring %q, and %d kernel OOM kills",
			numOOMObserved, substr, numKernelOOMObserved)
	}

	for {
		select {
		case <-timeoutCh:
			numKernelOOMObserved = kernelOOM.poll(readCtx)

			report()

			return numOOMObserved >= n || numKernelOOMObserved >= n
		case <-kernelPollTicker.C:
			numKernelOOMObserved = kernelOOM.poll(readCtx)

			// don't bail out early when n is zero, as in that case the point is to observe
			// the whole period and report what happened
			if n > 0 && numKernelOOMObserved >= n {
				report()

				return true
			}
		case <-timeToObserveCh:
			numKernelOOMObserved = kernelOOM.poll(readCtx)

			if numOOMObserved >= n || numKernelOOMObserved >= n {
				// if we already observed enough OOM kills, consider it a success
				report()

				return true
			}
		case ev := <-watchCh:
			if ev.Type != state.Created || ev.Resource.Metadata().Created().Before(startTime) {
				continue
			}

			res := ev.Resource.(*runtime.OOMAction).TypedSpec()

			matched, bailOut := matchOOMActionProcesses(res.Processes, substr)

			if matched {
				numOOMObserved++

				if numOOMObserved >= n {
					// if we already observed enough OOM events, consider it a success
					report()

					return true
				}
			}

			if bailOut {
				// the kernel OOM killer might have been the one doing the killing here,
				// so refresh its counters before declaring a failure
				numKernelOOMObserved = kernelOOM.poll(readCtx)

				suite.T().Logf("observed an OOM event not containing process substring %q: %v (%d containing, %d kernel OOM kills, ignoring it: %v)",
					substr, res.Processes, numOOMObserved, numKernelOOMObserved, allowNotMatchingKills)

				if !allowNotMatchingKills && numKernelOOMObserved < n {
					return false
				}
			}
		}
	}
}

// matchOOMActionProcesses inspects the processes killed in a single userspace OOM event.
//
// It reports whether the event contains a process matching substr, and whether it contains
// a process which is not expected to be killed at all.
func matchOOMActionProcesses(processes []string, substr string) (matched, bailOut bool) {
	for _, proc := range processes {
		if strings.Contains(proc, substr) {
			return true, bailOut
		}

		// Sometimes OOM catches containers in restart phase (while the
		// cgroup has previously accumulated OOM score).
		// Consider an OOM event wrong if something other than that is found.
		if !strings.Contains(proc, "runc init") && !strings.Contains(proc, "/pause") && proc != "" {
			bailOut = true
		}
	}

	return false, bailOut
}

// kernelOOMTracker tracks the number of kernel OOM kills across the worker nodes.
//
// Reads are best-effort: a node which fails to report its counter (which is likely, as the node
// is under memory pressure) keeps its last known value, so that the number of kills observed
// never goes down.
type kernelOOMTracker struct {
	suite    *OomSuite
	nodes    []string
	baseline map[string]int
	latest   map[string]int
}

// newKernelOOMTracker captures the baseline of the kernel OOM kill counters.
func (suite *OomSuite) newKernelOOMTracker(ctx context.Context, nodes []string) *kernelOOMTracker {
	baseline := suite.readKernelOOMCounters(ctx, nodes)

	return &kernelOOMTracker{
		suite:    suite,
		nodes:    nodes,
		baseline: baseline,
		latest:   maps.Clone(baseline),
	}
}

// poll refreshes the counters, returning the total number of kills observed since the baseline.
func (tracker *kernelOOMTracker) poll(ctx context.Context) int {
	for node, count := range tracker.suite.readKernelOOMCounters(ctx, tracker.nodes) {
		// a node without a baseline can't be counted, as the number of kills can't be established
		if _, ok := tracker.baseline[node]; ok {
			tracker.latest[node] = count
		}
	}

	var total int

	for node, count := range tracker.latest {
		total += count - tracker.baseline[node]
	}

	return total
}

// readKernelOOMCounters reads the cumulative kernel OOM kill counter from each node in parallel.
//
// Every read is bounded by kernelOOMReadTimeout, and nodes which fail to answer are skipped
// (with a log message) instead of failing the test.
func (suite *OomSuite) readKernelOOMCounters(ctx context.Context, nodes []string) map[string]int {
	ctx, cancel := context.WithTimeout(ctx, kernelOOMReadTimeout)
	defer cancel()

	counts := make([]int, len(nodes))
	errs := make([]error, len(nodes))

	var wg sync.WaitGroup

	for i, node := range nodes {
		wg.Go(func() {
			counts[i], errs[i] = suite.readKernelOOMCounter(client.WithNode(ctx, node))
		})
	}

	wg.Wait()

	counters := make(map[string]int, len(nodes))

	// the results are processed here (and not in the goroutines above) to keep the logging
	// on the goroutine running the test
	for i, node := range nodes {
		if errs[i] != nil {
			suite.T().Logf("failed to read kernel OOM kill counter from %s: %v", node, errs[i])

			continue
		}

		counters[node] = counts[i]
	}

	return counters
}

// readKernelOOMCounter reads the `oom_kill` counter from /proc/vmstat on a single node.
func (suite *OomSuite) readKernelOOMCounter(nodeCtx context.Context) (int, error) {
	reader, err := suite.Client.Read(nodeCtx, "/proc/vmstat")
	if err != nil {
		return 0, err
	}

	defer reader.Close() //nolint:errcheck

	contents, err := io.ReadAll(reader)
	if err != nil {
		return 0, err
	}

	for line := range strings.Lines(string(contents)) {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), "oom_kill ")
		if !ok {
			continue
		}

		count, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return 0, fmt.Errorf("failed to parse %q from /proc/vmstat: %w", line, err)
		}

		return count, nil
	}

	return 0, errors.New("oom_kill counter not found in /proc/vmstat")
}

func init() {
	allSuites = append(allSuites, new(OomSuite))
}
