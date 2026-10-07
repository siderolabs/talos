// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// workloadMemoryTree is a fake cgroup2 mount: the two Talos-owned roots plus the roots the
// controller must never touch, each seeded with a value distinct from anything a test writes.
type workloadMemoryTree struct {
	t    *testing.T
	root string
}

var workloadMemoryForeignRoots = map[string]string{
	constants.CgroupKubepods:       "17179869184\n",
	constants.CgroupSystem:         "1073741824\n",
	constants.CgroupInit:           "536870912\n",
	constants.CgroupPodRuntimeRoot: "2147483648\n",
}

func newWorkloadMemoryTree(t *testing.T, talosContainers, virtualMachines string) workloadMemoryTree {
	t.Helper()

	tree := workloadMemoryTree{t: t, root: t.TempDir()}

	tree.seed(constants.CgroupTalosContainersRoot, talosContainers)
	tree.seed(constants.CgroupVirtualMachines, virtualMachines)

	for name, contents := range workloadMemoryForeignRoots {
		tree.seed(name, contents)
	}

	return tree
}

func (tree workloadMemoryTree) path(name string) string {
	return filepath.Join(tree.root, name, "memory.max")
}

func (tree workloadMemoryTree) seed(name, contents string) {
	tree.t.Helper()

	require.NoError(tree.t, os.MkdirAll(filepath.Dir(tree.path(name)), 0o755))
	require.NoError(tree.t, os.WriteFile(tree.path(name), []byte(contents), 0o644))
}

func (tree workloadMemoryTree) read(name string) string {
	tree.t.Helper()

	contents, err := os.ReadFile(tree.path(name))
	require.NoError(tree.t, err)

	return string(contents)
}

func (tree workloadMemoryTree) assertRoots(talosContainers, virtualMachines string) {
	tree.t.Helper()

	assert.Equal(tree.t, talosContainers, tree.read(constants.CgroupTalosContainersRoot), "taloscontainers")
	assert.Equal(tree.t, virtualMachines, tree.read(constants.CgroupVirtualMachines), "virtualmachines.partition")
}

func (tree workloadMemoryTree) assertForeignRootsUntouched() {
	tree.t.Helper()

	for name, contents := range workloadMemoryForeignRoots {
		assert.Equal(tree.t, contents, tree.read(name), name)
	}
}

// workloadMemoryHarness runs the controller runtime inside a synctest bubble, so "nothing happened"
// can be asserted without sleeping: synctest.Wait returns once every goroutine is blocked.
type workloadMemoryHarness struct {
	ctest.DefaultSuite

	tree workloadMemoryTree
	logs *observer.ObservedLogs
}

func runWorkloadMemory(t *testing.T, tree workloadMemoryTree, mode machineruntime.Mode, fn func(h *workloadMemoryHarness)) {
	t.Helper()

	synctest.Test(t, func(t *testing.T) {
		core, logs := observer.New(zap.DebugLevel)

		h := &workloadMemoryHarness{tree: tree, logs: logs}
		h.Timeout = 30 * time.Second
		h.Logger = zap.New(zapcore.NewTee(core, zaptest.NewLogger(t).Core()))

		h.SetT(t)
		h.SetupTest()

		defer h.TearDownTest()

		h.Require().NoError(h.Runtime().RegisterController(&runtimectrls.WorkloadMemoryController{
			V1Alpha1Mode: mode,
			CgroupRoot:   tree.root,
		}))

		fn(h)

		tree.assertForeignRootsUntouched()
	})
}

func (h *workloadMemoryHarness) settle() {
	h.T().Helper()

	synctest.Wait()
}

func (h *workloadMemoryHarness) createSpec(spec runtime.WorkloadMemorySpecSpec) {
	h.T().Helper()

	res := runtime.NewWorkloadMemorySpec()
	*res.TypedSpec() = spec

	h.Create(res)
	h.settle()
}

func (h *workloadMemoryHarness) updateSpec(spec runtime.WorkloadMemorySpecSpec) {
	h.T().Helper()

	ctest.UpdateWithConflicts(h, runtime.NewWorkloadMemorySpec(), func(res *runtime.WorkloadMemorySpec) error {
		*res.TypedSpec() = spec

		return nil
	})
	h.settle()
}

func (h *workloadMemoryHarness) destroySpec() {
	h.T().Helper()

	spec, err := safe.StateGetByID[*runtime.WorkloadMemorySpec](h.Ctx(), h.State(), runtime.WorkloadMemorySpecID)
	h.Require().NoError(err)
	h.Destroy(spec)
	h.settle()
}

// writes returns the number of memory.max updates the controller has logged for the root.
func (h *workloadMemoryHarness) writes(name string) int {
	return h.logs.FilterMessage("memory limit updated").Filter(func(entry observer.LoggedEntry) bool {
		return entry.ContextMap()["cgroup"] == name
	}).Len()
}

// failures returns the controller failure messages seen so far.
func (h *workloadMemoryHarness) failures() []string {
	var out []string

	for _, entry := range h.logs.FilterMessage("controller failed").All() {
		if err, ok := entry.ContextMap()["error"].(string); ok {
			out = append(out, err)
		}
	}

	return out
}

func (h *workloadMemoryHarness) assertNoFailures() {
	h.T().Helper()

	h.Assert().Empty(h.failures())
}

func gib(n uint64) string {
	return strconv.FormatUint(n<<30, 10)
}

func TestWorkloadMemoryMissingSpecWaitsThenDefaultResets(t *testing.T) {
	t.Parallel()

	tree := newWorkloadMemoryTree(t, gib(1)+"\n", "max\n")

	runWorkloadMemory(t, tree, machineruntime.ModeMetal, func(h *workloadMemoryHarness) {
		h.settle()
		tree.assertRoots(gib(1)+"\n", "max\n")
		h.Require().Zero(h.writes(constants.CgroupTalosContainersRoot))

		h.createSpec(runtime.WorkloadMemorySpecSpec{})
		tree.assertRoots("max", "max\n")
		h.Assert().Zero(h.writes(constants.CgroupVirtualMachines))

		updates := h.logs.FilterMessage("memory limit updated").All()
		h.Require().Len(updates, 1)
		h.Assert().Equal(constants.CgroupTalosContainersRoot, updates[0].ContextMap()["cgroup"])
		h.Assert().Equal(strconv.FormatUint(1<<30, 10), updates[0].ContextMap()["previous"])
		h.Assert().Equal("max", updates[0].ContextMap()["limit"])

		h.assertNoFailures()
	})
}

func TestWorkloadMemorySetChangeRemove(t *testing.T) {
	t.Parallel()

	tree := newWorkloadMemoryTree(t, "max\n", "max\n")

	runWorkloadMemory(t, tree, machineruntime.ModeMetal, func(h *workloadMemoryHarness) {
		h.createSpec(runtime.WorkloadMemorySpecSpec{TalosContainersLimit: 4 << 30, VirtualMachinesLimit: 32 << 30})
		tree.assertRoots(gib(4), gib(32))

		h.updateSpec(runtime.WorkloadMemorySpecSpec{TalosContainersLimit: 2 << 30, VirtualMachinesLimit: 32 << 30})
		tree.assertRoots(gib(2), gib(32))
		h.Assert().Equal(2, h.writes(constants.CgroupTalosContainersRoot))
		h.Assert().Equal(1, h.writes(constants.CgroupVirtualMachines))

		h.updateSpec(runtime.WorkloadMemorySpecSpec{VirtualMachinesLimit: 32 << 30})
		tree.assertRoots("max", gib(32))

		h.updateSpec(runtime.WorkloadMemorySpecSpec{})
		tree.assertRoots("max", "max")
		h.Assert().Equal(3, h.writes(constants.CgroupTalosContainersRoot))
		h.Assert().Equal(2, h.writes(constants.CgroupVirtualMachines))

		// the spec disappearing (projection withdrawn) leaves the roots as they are
		h.destroySpec()
		tree.assertRoots("max", "max")
		h.Assert().Equal(3, h.writes(constants.CgroupTalosContainersRoot))

		h.assertNoFailures()
	})
}

func TestWorkloadMemorySameDesiredAvoidsWrites(t *testing.T) {
	t.Parallel()

	tree := newWorkloadMemoryTree(t, "max\n", gib(32)+"\n")

	runWorkloadMemory(t, tree, machineruntime.ModeMetal, func(h *workloadMemoryHarness) {
		spec := runtime.WorkloadMemorySpecSpec{TalosContainersLimit: 4 << 30, VirtualMachinesLimit: 32 << 30}

		h.createSpec(spec)
		tree.assertRoots(gib(4), gib(32)+"\n")
		h.Assert().Equal(1, h.writes(constants.CgroupTalosContainersRoot))
		h.Assert().Zero(h.writes(constants.CgroupVirtualMachines))

		h.Require().NoError(os.Chmod(tree.path(constants.CgroupTalosContainersRoot), 0o444))
		h.Require().NoError(os.Chmod(tree.path(constants.CgroupVirtualMachines), 0o444))

		h.updateSpec(spec)
		h.updateSpec(spec)
		tree.assertRoots(gib(4), gib(32)+"\n")
		h.Assert().Equal(1, h.writes(constants.CgroupTalosContainersRoot))
		h.Assert().Zero(h.writes(constants.CgroupVirtualMachines))

		h.assertNoFailures()
	})
}

func TestWorkloadMemoryRemovalThenRestartResets(t *testing.T) {
	t.Parallel()

	tree := newWorkloadMemoryTree(t, "max\n", "max\n")

	runWorkloadMemory(t, tree, machineruntime.ModeMetal, func(h *workloadMemoryHarness) {
		h.createSpec(runtime.WorkloadMemorySpecSpec{TalosContainersLimit: 4 << 30, VirtualMachinesLimit: 32 << 30})
		tree.assertRoots(gib(4), gib(32))
		h.assertNoFailures()
	})

	// fresh runtime over the same roots: the document is gone, so the projection is the default spec
	runWorkloadMemory(t, tree, machineruntime.ModeMetal, func(h *workloadMemoryHarness) {
		h.createSpec(runtime.WorkloadMemorySpecSpec{})
		tree.assertRoots("max", "max")
		h.Assert().Equal(1, h.writes(constants.CgroupTalosContainersRoot))
		h.Assert().Equal(1, h.writes(constants.CgroupVirtualMachines))
		h.assertNoFailures()
	})
}

func TestWorkloadMemoryOneRootFailingDoesNotBlockOther(t *testing.T) {
	t.Parallel()

	tree := newWorkloadMemoryTree(t, "max\n", "max\n")

	require.NoError(t, os.Remove(tree.path(constants.CgroupVirtualMachines)))

	runWorkloadMemory(t, tree, machineruntime.ModeMetal, func(h *workloadMemoryHarness) {
		h.createSpec(runtime.WorkloadMemorySpecSpec{TalosContainersLimit: 4 << 30, VirtualMachinesLimit: 32 << 30})
		h.Assert().Equal(gib(4), tree.read(constants.CgroupTalosContainersRoot))
		h.Assert().Equal(1, h.writes(constants.CgroupTalosContainersRoot))

		failures := h.failures()
		h.Require().Len(failures, 1)
		h.Assert().Contains(failures[0], constants.CgroupVirtualMachines)
		h.Assert().Contains(failures[0], "no such file or directory")
		h.Assert().NotContains(failures[0], constants.CgroupTalosContainersRoot)

		// the controller is in restart backoff; once the root exists the retry applies the limit
		tree.seed(constants.CgroupVirtualMachines, "max\n")

		time.Sleep(time.Minute)
		h.settle()

		tree.assertRoots(gib(4), gib(32))
		h.Assert().Equal(1, h.writes(constants.CgroupTalosContainersRoot))
		h.Assert().Equal(1, h.writes(constants.CgroupVirtualMachines))
		h.Assert().Len(h.failures(), 1)
	})
}

func TestWorkloadMemoryBothRootsFailingAggregates(t *testing.T) {
	t.Parallel()

	tree := newWorkloadMemoryTree(t, "max\n", "max\n")

	require.NoError(t, os.Remove(tree.path(constants.CgroupTalosContainersRoot)))
	require.NoError(t, os.Remove(tree.path(constants.CgroupVirtualMachines)))

	runWorkloadMemory(t, tree, machineruntime.ModeMetal, func(h *workloadMemoryHarness) {
		h.createSpec(runtime.WorkloadMemorySpecSpec{TalosContainersLimit: 4 << 30})

		failures := h.failures()
		h.Require().Len(failures, 1)
		h.Assert().Contains(failures[0], constants.CgroupTalosContainersRoot)
		h.Assert().Contains(failures[0], constants.CgroupVirtualMachines)
	})
}

func TestWorkloadMemoryNormalization(t *testing.T) {
	t.Parallel()

	pageSize := uint64(os.Getpagesize())

	tree := newWorkloadMemoryTree(t, "max\n", "max\n")

	runWorkloadMemory(t, tree, machineruntime.ModeMetal, func(h *workloadMemoryHarness) {
		h.createSpec(runtime.WorkloadMemorySpecSpec{TalosContainersLimit: 4<<30 + pageSize - 1, VirtualMachinesLimit: pageSize})
		tree.assertRoots(gib(4), strconv.FormatUint(pageSize, 10))

		// a different raw value with the same page-rounded result is not a change
		h.Require().NoError(os.Chmod(tree.path(constants.CgroupTalosContainersRoot), 0o444))

		h.updateSpec(runtime.WorkloadMemorySpecSpec{TalosContainersLimit: 4<<30 + 1, VirtualMachinesLimit: pageSize})
		tree.assertRoots(gib(4), strconv.FormatUint(pageSize, 10))
		h.Assert().Equal(1, h.writes(constants.CgroupTalosContainersRoot))

		h.assertNoFailures()
	})
}

func TestWorkloadMemoryRejectedLimitsLeaveRootsUntouched(t *testing.T) {
	t.Parallel()

	pageSize := uint64(os.Getpagesize())

	for _, test := range []struct {
		name   string
		limit  uint64
		errMsg string
	}{
		{name: "sub page", limit: pageSize - 1, errMsg: "smaller than the page size"},
		{name: "int64 max", limit: math.MaxInt64, errMsg: "treated as unlimited"},
		{name: "page counter max", limit: (math.MaxInt64 / pageSize) * pageSize, errMsg: "treated as unlimited"},
		{name: "exceeds int64", limit: math.MaxInt64 + 1, errMsg: "exceeds the maximum"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// a limit which is already in place must stay rather than being lifted to "max"
			tree := newWorkloadMemoryTree(t, gib(1)+"\n", gib(8)+"\n")

			runWorkloadMemory(t, tree, machineruntime.ModeMetal, func(h *workloadMemoryHarness) {
				h.createSpec(runtime.WorkloadMemorySpecSpec{TalosContainersLimit: test.limit, VirtualMachinesLimit: 2 << 30})
				tree.assertRoots(gib(1)+"\n", gib(2))
				h.Assert().Zero(h.writes(constants.CgroupTalosContainersRoot))
				h.Assert().Equal(1, h.writes(constants.CgroupVirtualMachines))

				failures := h.failures()
				h.Require().Len(failures, 1)
				h.Assert().Contains(failures[0], constants.CgroupTalosContainersRoot)
				h.Assert().Contains(failures[0], test.errMsg)
				h.Assert().Contains(failures[0], strconv.FormatUint(test.limit, 10))
			})
		})
	}
}

func TestWorkloadMemoryUnparseableRootIsNotOverwritten(t *testing.T) {
	t.Parallel()

	// a plain file reads back exactly what was written, so the post-write readback diagnostic is
	// exercised against a real kernel in cgroup.TestMemoryMaxKernel; here the pre-write read fails
	tree := newWorkloadMemoryTree(t, "garbage\n", "max\n")

	runWorkloadMemory(t, tree, machineruntime.ModeMetal, func(h *workloadMemoryHarness) {
		h.createSpec(runtime.WorkloadMemorySpecSpec{TalosContainersLimit: 4 << 30, VirtualMachinesLimit: 2 << 30})
		tree.assertRoots("garbage\n", gib(2))

		failures := h.failures()
		h.Require().Len(failures, 1)
		h.Assert().Contains(failures[0], constants.CgroupTalosContainersRoot)
		h.Assert().Contains(failures[0], `unexpected value "garbage"`)
	})
}

func TestWorkloadMemoryContainerModeNeverTouchesRoots(t *testing.T) {
	t.Parallel()

	tree := newWorkloadMemoryTree(t, gib(1)+"\n", "max\n")

	for name := range workloadMemoryForeignRoots {
		require.NoError(t, os.Chmod(tree.path(name), 0o444))
	}

	require.NoError(t, os.Chmod(tree.path(constants.CgroupTalosContainersRoot), 0o444))
	require.NoError(t, os.Chmod(tree.path(constants.CgroupVirtualMachines), 0o444))

	runWorkloadMemory(t, tree, machineruntime.ModeContainer, func(h *workloadMemoryHarness) {
		h.createSpec(runtime.WorkloadMemorySpecSpec{TalosContainersLimit: 4 << 30, VirtualMachinesLimit: 32 << 30})
		tree.assertRoots(gib(1)+"\n", "max\n")

		h.updateSpec(runtime.WorkloadMemorySpecSpec{})
		tree.assertRoots(gib(1)+"\n", "max\n")

		h.Assert().Zero(h.logs.FilterMessage("memory limit updated").Len())
		h.assertNoFailures()
	})
}

func TestWorkloadMemoryControllerDefinition(t *testing.T) {
	t.Parallel()

	ctrl := &runtimectrls.WorkloadMemoryController{}

	assert.Equal(t, "runtime.WorkloadMemoryController", ctrl.Name())
	assert.Nil(t, ctrl.Outputs())

	inputs := ctrl.Inputs()
	require.Len(t, inputs, 1)
	assert.Equal(t, runtime.WorkloadMemorySpecType, inputs[0].Type)
}
