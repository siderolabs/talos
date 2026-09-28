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
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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

		// Wait for all stress-mem pods to complete terminating
		if !suite.waitForStressPodsGone(cleanUpCtx) {
			suite.Require().Fail("Timed out waiting for cleanup")

			return
		}

		// Memory pressure on the worker nodes doesn't go away the moment the stress pods are gone:
		// the OOM handler trigger is based on a 10-second PSI average, and the kernel might still be
		// reclaiming memory. If the next test suite deploys something memory-hungry right away, the
		// OOM handler picks it as the next victim (see waitForMemoryPressureToSubside), so wait for the
		// nodes to settle before handing over.
		settleCtx, settleCancel := context.WithTimeout(context.Background(), memoryPressureSettleTimeout)
		defer settleCancel()

		if !suite.waitForMemoryPressureToSubside(settleCtx, suite.DiscoverNodeInternalIPsByType(settleCtx, machine.TypeWorker)) {
			suite.Assert().Failf("memory pressure didn't subside", "worker nodes are still under memory pressure %s after the stress pods were removed", memoryPressureSettleTimeout)
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

// waitForStressPodsGone polls until no stress-mem pods are left, returning false if ctx expires first.
func (suite *OomSuite) waitForStressPodsGone(ctx context.Context) bool {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pods, err := suite.Clientset.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
				LabelSelector: "app=stress-mem",
			})

			suite.Require().NoError(err)

			if len(pods.Items) == 0 {
				return true
			}
		case <-ctx.Done():
			return false
		}
	}
}

const (
	// memoryPressureSettleTimeout bounds the wait for memory pressure to subside on the worker nodes
	// after the stress pods are removed.
	memoryPressureSettleTimeout = 3 * time.Minute

	// memoryPressureQuietPeriod is how long every worker node has to stay below the pressure threshold
	// (with no new userspace OOM kills) before the nodes are considered settled.
	memoryPressureQuietPeriod = 15 * time.Second

	// memoryPressureFullAvg10Threshold is the upper bound on the PSI memory "full avg10" value for a node
	// to be considered free of memory pressure.
	//
	// The default OOM handler trigger (constants.DefaultOOMTriggerExpression) fires when the sum of "full avg10"
	// over the system and podruntime cgroups exceeds 5.0, so both that sum and the root cgroup value are
	// required to stay well below it.
	memoryPressureFullAvg10Threshold = 1.0
)

// memoryPressureCgroups lists the cgroups (relative to the cgroup root) sampled by the OOM handler trigger
// expression, plus the root cgroup which covers everything else (e.g. the pods).
var memoryPressureCgroups = []string{
	"",
	constants.CgroupInit,
	constants.CgroupSystem,
	constants.CgroupPodRuntimeRoot,
}

// waitForMemoryPressureToSubside waits until every node stays below the memory pressure threshold for
// memoryPressureQuietPeriod, with no new userspace OOM kills happening in the meantime.
//
// It returns false if ctx expires before the nodes settle.
//
// A node which fails to report its state (which is likely while it's under memory pressure) is considered
// to be under pressure.
func (suite *OomSuite) waitForMemoryPressureToSubside(ctx context.Context, nodes []string) bool {
	startTime := time.Now()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var quietSince time.Time

	for {
		select {
		case <-ctx.Done():
			suite.T().Logf("memory pressure didn't subside on the worker nodes in %s", time.Since(startTime))

			return false
		case <-ticker.C:
		}

		if reason := suite.memoryPressureReason(ctx, nodes, startTime); reason != "" {
			if !quietSince.IsZero() {
				suite.T().Logf("worker nodes are back under memory pressure: %s", reason)
			}

			quietSince = time.Time{}

			continue
		}

		if quietSince.IsZero() {
			quietSince = time.Now()
		}

		if time.Since(quietSince) >= memoryPressureQuietPeriod {
			suite.T().Logf("memory pressure on the worker nodes subsided in %s", time.Since(startTime))

			return true
		}
	}
}

// memoryPressureReason checks all the nodes in parallel, returning an empty string if none of them is
// under memory pressure, or a human-readable reason otherwise.
func (suite *OomSuite) memoryPressureReason(ctx context.Context, nodes []string, since time.Time) string {
	ctx, cancel := context.WithTimeout(ctx, kernelOOMReadTimeout)
	defer cancel()

	reasons := make([]string, len(nodes))

	var wg sync.WaitGroup

	for i, node := range nodes {
		wg.Go(func() {
			reasons[i] = suite.nodeMemoryPressureReason(client.WithNode(ctx, node), since)
		})
	}

	wg.Wait()

	var nonEmpty []string

	for i, node := range nodes {
		if reasons[i] != "" {
			nonEmpty = append(nonEmpty, node+": "+reasons[i])
		}
	}

	return strings.Join(nonEmpty, "; ")
}

// nodeMemoryPressureReason checks a single node for memory pressure.
func (suite *OomSuite) nodeMemoryPressureReason(nodeCtx context.Context, since time.Time) string {
	if reason := suite.recentOOMActionReason(nodeCtx, since); reason != "" {
		return reason
	}

	return suite.psiMemoryPressureReason(nodeCtx)
}

// recentOOMActionReason reports a userspace OOM kill on the node since the given time, if any.
//
// Any such kill means the OOM handler still considers the node to be under memory pressure.
func (suite *OomSuite) recentOOMActionReason(nodeCtx context.Context, since time.Time) string {
	actions, err := safe.StateListAll[*runtime.OOMAction](nodeCtx, suite.Client.COSI)
	if err != nil {
		return fmt.Sprintf("failed to list OOM actions: %v", err)
	}

	for action := range actions.All() {
		if !action.Metadata().Created().Before(since) {
			return fmt.Sprintf("userspace OOM kill at %s: %v", action.Metadata().Created().Format(time.RFC3339), action.TypedSpec().Processes)
		}
	}

	return ""
}

// psiMemoryPressureReason reports the node's PSI memory pressure if it's above the threshold.
func (suite *OomSuite) psiMemoryPressureReason(nodeCtx context.Context) string {
	var (
		rootFullAvg10    float64
		triggerFullAvg10 float64
	)

	for _, cgroup := range memoryPressureCgroups {
		fullAvg10, err := suite.readMemoryPressureFullAvg10(nodeCtx, cgroup)
		if err != nil {
			return fmt.Sprintf("failed to read memory pressure of cgroup %q: %v", cgroup, err)
		}

		if cgroup == "" {
			rootFullAvg10 = fullAvg10
		} else {
			// mirrors the sum over the system and podruntime classes in the default trigger expression
			triggerFullAvg10 += fullAvg10
		}
	}

	if rootFullAvg10 >= memoryPressureFullAvg10Threshold || triggerFullAvg10 >= memoryPressureFullAvg10Threshold {
		return fmt.Sprintf("memory PSI full avg10: root %.2f, system+podruntime %.2f", rootFullAvg10, triggerFullAvg10)
	}

	return ""
}

// readMemoryPressureFullAvg10 reads the "full avg10" PSI value of the cgroup's memory.pressure file on the node.
func (suite *OomSuite) readMemoryPressureFullAvg10(nodeCtx context.Context, cgroup string) (float64, error) {
	reader, err := suite.Client.Read(nodeCtx, filepath.Join(constants.CgroupMountPath, cgroup, "memory.pressure"))
	if err != nil {
		return 0, err
	}

	defer reader.Close() //nolint:errcheck

	contents, err := io.ReadAll(reader)
	if err != nil {
		return 0, err
	}

	return parsePSIFullAvg10(string(contents))
}

// parsePSIFullAvg10 extracts the "full avg10" value out of the contents of a PSI file, e.g.:
//
//	some avg10=0.00 avg60=0.00 avg300=0.00 total=0
//	full avg10=0.00 avg60=0.00 avg300=0.00 total=0
func parsePSIFullAvg10(contents string) (float64, error) {
	for line := range strings.Lines(contents) {
		fields := strings.Fields(line)

		if len(fields) < 2 || fields[0] != "full" {
			continue
		}

		for _, field := range fields[1:] {
			value, ok := strings.CutPrefix(field, "avg10=")
			if !ok {
				continue
			}

			avg10, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse %q: %w", line, err)
			}

			return avg10, nil
		}
	}

	return 0, errors.New("full avg10 not found in PSI contents")
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
