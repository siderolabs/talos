// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/synctest"
	"time"

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

// workloadPidsForeignRoots are roots whose pids.max the controller must never touch.
var workloadPidsForeignRoots = map[string]string{
	constants.CgroupKubepods:        "261044\n",
	constants.CgroupVirtualMachines: "max\n",
	constants.CgroupSystem:          "max\n",
	constants.CgroupPodRuntimeRoot:  "max\n",
}

// workloadPidsHost is a fake cgroup2 mount plus a fake /proc/sys.
type workloadPidsHost struct {
	t       *testing.T
	cgroups string
	procSys string
}

func newWorkloadPidsHost(t *testing.T, talosContainers, pidMax, threadsMax string) workloadPidsHost {
	t.Helper()

	host := workloadPidsHost{t: t, cgroups: t.TempDir(), procSys: t.TempDir()}

	host.seed(constants.CgroupTalosContainersRoot, talosContainers)

	for name, contents := range workloadPidsForeignRoots {
		host.seed(name, contents)
	}

	require.NoError(t, os.MkdirAll(filepath.Join(host.procSys, "kernel"), 0o755))
	host.setSysctl("pid_max", pidMax)
	host.setSysctl("threads-max", threadsMax)

	return host
}

func (host workloadPidsHost) path(name string) string {
	return filepath.Join(host.cgroups, name, "pids.max")
}

func (host workloadPidsHost) seed(name, contents string) {
	host.t.Helper()

	require.NoError(host.t, os.MkdirAll(filepath.Dir(host.path(name)), 0o755))
	require.NoError(host.t, os.WriteFile(host.path(name), []byte(contents), 0o644))
}

func (host workloadPidsHost) read(name string) string {
	host.t.Helper()

	contents, err := os.ReadFile(host.path(name))
	require.NoError(host.t, err)

	return string(contents)
}

func (host workloadPidsHost) setSysctl(name, value string) {
	host.t.Helper()

	require.NoError(host.t, os.WriteFile(filepath.Join(host.procSys, "kernel", name), []byte(value+"\n"), 0o644))
}

// assertForeignRootsUntouched checks every seeded foreign root which the controller was not configured with.
func (host workloadPidsHost) assertForeignRootsUntouched(roots []runtimectrls.WorkloadPidsRoot) {
	host.t.Helper()

	for name, contents := range workloadPidsForeignRoots {
		if slices.ContainsFunc(roots, func(root runtimectrls.WorkloadPidsRoot) bool { return root.Cgroup == name }) {
			continue
		}

		assert.Equal(host.t, contents, host.read(name), name)
	}
}

type workloadPidsHarness struct {
	ctest.DefaultSuite

	host workloadPidsHost
	logs *observer.ObservedLogs
}

var talosContainersPidsRoot = runtimectrls.WorkloadPidsRoot{
	Cgroup:       constants.CgroupTalosContainersRoot,
	ReservedPids: constants.CgroupTalosContainersReservedPids,
}

func runWorkloadPids(t *testing.T, host workloadPidsHost, mode machineruntime.Mode, fn func(h *workloadPidsHarness)) {
	t.Helper()

	runWorkloadPidsRoots(t, host, mode, []runtimectrls.WorkloadPidsRoot{talosContainersPidsRoot}, fn)
}

func runWorkloadPidsRoots(
	t *testing.T, host workloadPidsHost, mode machineruntime.Mode, roots []runtimectrls.WorkloadPidsRoot, fn func(h *workloadPidsHarness),
) {
	t.Helper()

	synctest.Test(t, func(t *testing.T) {
		core, logs := observer.New(zap.DebugLevel)

		h := &workloadPidsHarness{host: host, logs: logs}
		h.Timeout = 30 * time.Second
		h.Logger = zap.New(zapcore.NewTee(core, zaptest.NewLogger(t).Core()))

		h.SetT(t)
		h.SetupTest()

		defer h.TearDownTest()

		h.Require().NoError(h.Runtime().RegisterController(&runtimectrls.WorkloadPidsController{
			V1Alpha1Mode: mode,
			Roots:        roots,
			CgroupRoot:   host.cgroups,
			ProcSysRoot:  host.procSys,
		}))

		synctest.Wait()

		fn(h)

		host.assertForeignRootsUntouched(roots)
	})
}

// sysctlApplied mimics KernelParamSpecController: the sysctl is written, then its status is published.
func (h *workloadPidsHarness) sysctlApplied(name, value string) {
	h.T().Helper()

	h.host.setSysctl(name, value)

	ctest.UpdateWithConflicts(
		h, runtime.NewKernelParamStatus(runtime.NamespaceName, "kernel."+name),
		func(res *runtime.KernelParamStatus) error {
			res.TypedSpec().Current = value

			return nil
		},
	)

	synctest.Wait()
}

func (h *workloadPidsHarness) createStatus(name, value string) {
	h.T().Helper()

	h.host.setSysctl(name, value)

	status := runtime.NewKernelParamStatus(runtime.NamespaceName, "kernel."+name)
	status.TypedSpec().Current = value

	h.Create(status)
	synctest.Wait()
}

func (h *workloadPidsHarness) writes() int {
	return h.logs.FilterMessage("pids limit updated").Len()
}

// rootWrites returns the number of pids.max updates the controller has logged for the root.
func (h *workloadPidsHarness) rootWrites(name string) int {
	return h.logs.FilterMessage("pids limit updated").Filter(func(entry observer.LoggedEntry) bool {
		return entry.ContextMap()["cgroup"] == name
	}).Len()
}

func (h *workloadPidsHarness) failures() []string {
	var out []string

	for _, entry := range h.logs.FilterMessage("controller failed").All() {
		if err, ok := entry.ContextMap()["error"].(string); ok {
			out = append(out, err)
		}
	}

	return out
}

func TestWorkloadPidsDefault(t *testing.T) {
	t.Parallel()

	host := newWorkloadPidsHost(t, "max\n", "262144", "500000")

	runWorkloadPids(t, host, machineruntime.ModeMetal, func(h *workloadPidsHarness) {
		// the startup event alone applies the limit, before any kernel parameter status exists
		h.Assert().Equal("262044", host.read(constants.CgroupTalosContainersRoot))

		updates := h.logs.FilterMessage("pids limit updated").All()
		h.Require().Len(updates, 1)
		h.Assert().Equal(constants.CgroupTalosContainersRoot, updates[0].ContextMap()["cgroup"])
		h.Assert().Equal("max", updates[0].ContextMap()["previous"])
		h.Assert().Equal("262044", updates[0].ContextMap()["limit"])

		h.Assert().Empty(h.failures())
	})
}

func TestWorkloadPidsThreadsMaxLower(t *testing.T) {
	t.Parallel()

	host := newWorkloadPidsHost(t, "max\n", "262144", "15000")

	runWorkloadPids(t, host, machineruntime.ModeMetal, func(h *workloadPidsHarness) {
		h.Assert().Equal("14900", host.read(constants.CgroupTalosContainersRoot))
		h.Assert().Empty(h.failures())
	})
}

func TestWorkloadPidsFollowsSysctlChanges(t *testing.T) {
	t.Parallel()

	host := newWorkloadPidsHost(t, "max\n", "4194304", "500000")

	runWorkloadPids(t, host, machineruntime.ModeMetal, func(h *workloadPidsHarness) {
		h.Assert().Equal("499900", host.read(constants.CgroupTalosContainersRoot))

		// Talos default kernel.pid_max lands
		h.createStatus("pid_max", "262144")
		h.Assert().Equal("262044", host.read(constants.CgroupTalosContainersRoot))

		// user lowers it via machine.sysctls
		h.sysctlApplied("pid_max", "131072")
		h.Assert().Equal("130972", host.read(constants.CgroupTalosContainersRoot))

		// threads-max becomes the binding limit
		h.createStatus("threads-max", "20000")
		h.Assert().Equal("19900", host.read(constants.CgroupTalosContainersRoot))

		h.Assert().Equal(4, h.writes())
		h.Assert().Empty(h.failures())
	})
}

func TestWorkloadPidsLeavesRootUntouched(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		mode    machineruntime.Mode
		initial string
		sysctl  string
		value   string
	}{
		{
			// an unrelated sysctl changing does not change the capacity, and the limit is already in place
			name:    "same desired value",
			mode:    machineruntime.ModeMetal,
			initial: "262044\n",
			sysctl:  "panic",
			value:   "10",
		},
		{
			// the capacity changes, but cgroups are nested in container mode
			name:    "container mode",
			mode:    machineruntime.ModeContainer,
			initial: "max\n",
			sysctl:  "pid_max",
			value:   "131072",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			host := newWorkloadPidsHost(t, test.initial, "262144", "500000")

			// read-only: any write attempt would surface as a controller failure
			require.NoError(t, os.Chmod(host.path(constants.CgroupTalosContainersRoot), 0o444))

			runWorkloadPids(t, host, test.mode, func(h *workloadPidsHarness) {
				h.createStatus(test.sysctl, test.value)

				h.Assert().Equal(test.initial, host.read(constants.CgroupTalosContainersRoot))
				h.Assert().Zero(h.writes())
				h.Assert().Empty(h.failures())
			})
		})
	}
}

func TestWorkloadPidsRestoresDrift(t *testing.T) {
	t.Parallel()

	host := newWorkloadPidsHost(t, "max\n", "262144", "500000")

	runWorkloadPids(t, host, machineruntime.ModeMetal, func(h *workloadPidsHarness) {
		h.Assert().Equal("262044", host.read(constants.CgroupTalosContainersRoot))

		host.seed(constants.CgroupTalosContainersRoot, "max\n")

		h.createStatus("panic", "10")
		h.Assert().Equal("262044", host.read(constants.CgroupTalosContainersRoot))
		h.Assert().Equal(2, h.writes())
	})
}

func TestWorkloadPidsCapacityBelowReservation(t *testing.T) {
	t.Parallel()

	host := newWorkloadPidsHost(t, "max\n", "262144", "50")

	runWorkloadPids(t, host, machineruntime.ModeMetal, func(h *workloadPidsHarness) {
		// floored at zero, as kubelet Node Allocatable
		h.Assert().Equal("0", host.read(constants.CgroupTalosContainersRoot))

		warnings := h.logs.FilterLevelExact(zap.WarnLevel).FilterField(zap.Uint64("capacity", 50)).All()
		h.Require().Len(warnings, 1)
		h.Assert().Equal(constants.CgroupTalosContainersRoot, warnings[0].ContextMap()["cgroup"])
	})
}

func TestWorkloadPidsMalformedSysctlLeavesRootUntouched(t *testing.T) {
	t.Parallel()

	host := newWorkloadPidsHost(t, "max\n", "lots", "500000")

	runWorkloadPids(t, host, machineruntime.ModeMetal, func(h *workloadPidsHarness) {
		h.Assert().Equal("max\n", host.read(constants.CgroupTalosContainersRoot))

		failures := h.failures()
		h.Require().Len(failures, 1)
		h.Assert().Contains(failures[0], "error reading PID capacity")
		h.Assert().Contains(failures[0], "pid_max")

		// the controller is in restart backoff; once the sysctl is sane the retry applies the limit
		host.setSysctl("pid_max", "262144")

		time.Sleep(time.Minute)
		synctest.Wait()

		h.Assert().Equal("262044", host.read(constants.CgroupTalosContainersRoot))
		h.Assert().Len(h.failures(), 1)
	})
}

func TestWorkloadPidsMissingRootFails(t *testing.T) {
	t.Parallel()

	host := newWorkloadPidsHost(t, "max\n", "262144", "500000")
	require.NoError(t, os.Remove(host.path(constants.CgroupTalosContainersRoot)))

	runWorkloadPids(t, host, machineruntime.ModeMetal, func(h *workloadPidsHarness) {
		failures := h.failures()
		h.Require().Len(failures, 1)
		h.Assert().Contains(failures[0], constants.CgroupTalosContainersRoot)
		h.Assert().Contains(failures[0], "no such file or directory")
	})
}

// TestWorkloadPidsTalosRoots runs the controller with the roots machined registers.
func TestWorkloadPidsTalosRoots(t *testing.T) {
	t.Parallel()

	host := newWorkloadPidsHost(t, "max\n", "262144", "15000")

	roots := []runtimectrls.WorkloadPidsRoot{
		talosContainersPidsRoot,
		{Cgroup: constants.CgroupVirtualMachines, ReservedPids: constants.CgroupVirtualMachinesReservedPids},
	}

	runWorkloadPidsRoots(t, host, machineruntime.ModeMetal, roots, func(h *workloadPidsHarness) {
		h.Assert().Equal("14900", host.read(constants.CgroupTalosContainersRoot))
		h.Assert().Equal("14900", host.read(constants.CgroupVirtualMachines))

		h.createStatus("threads-max", "40000")
		h.Assert().Equal("39900", host.read(constants.CgroupTalosContainersRoot))
		h.Assert().Equal("39900", host.read(constants.CgroupVirtualMachines))

		h.Assert().Empty(h.failures())
	})

	// kubepods stays with the kubelet
	assert.Equal(t, workloadPidsForeignRoots[constants.CgroupKubepods], host.read(constants.CgroupKubepods))
}

// workloadPidsOtherRoot is a second, hypothetical root: the controller is not tied to taloscontainers.
const workloadPidsOtherRoot = "otherworkloads"

func TestWorkloadPidsMultipleRootsWithOwnReservations(t *testing.T) {
	t.Parallel()

	host := newWorkloadPidsHost(t, "max\n", "262144", "500000")
	host.seed(workloadPidsOtherRoot, "max\n")

	roots := []runtimectrls.WorkloadPidsRoot{
		talosContainersPidsRoot,
		{Cgroup: workloadPidsOtherRoot, ReservedPids: 1000},
	}

	runWorkloadPidsRoots(t, host, machineruntime.ModeMetal, roots, func(h *workloadPidsHarness) {
		h.Assert().Equal("262044", host.read(constants.CgroupTalosContainersRoot))
		h.Assert().Equal("261144", host.read(workloadPidsOtherRoot))

		// a single capacity change moves every root
		h.createStatus("pid_max", "131072")
		h.Assert().Equal("130972", host.read(constants.CgroupTalosContainersRoot))
		h.Assert().Equal("130072", host.read(workloadPidsOtherRoot))

		h.Assert().Equal(2, h.rootWrites(constants.CgroupTalosContainersRoot))
		h.Assert().Equal(2, h.rootWrites(workloadPidsOtherRoot))
		h.Assert().Empty(h.failures())
	})
}

func TestWorkloadPidsOneRootFailingDoesNotBlockOther(t *testing.T) {
	t.Parallel()

	host := newWorkloadPidsHost(t, "max\n", "262144", "500000")

	roots := []runtimectrls.WorkloadPidsRoot{
		{Cgroup: workloadPidsOtherRoot, ReservedPids: 1000},
		talosContainersPidsRoot,
	}

	runWorkloadPidsRoots(t, host, machineruntime.ModeMetal, roots, func(h *workloadPidsHarness) {
		// the failing root comes first, yet the next one is still enforced
		h.Assert().Equal("262044", host.read(constants.CgroupTalosContainersRoot))

		failures := h.failures()
		h.Require().Len(failures, 1)
		h.Assert().Contains(failures[0], workloadPidsOtherRoot)
		h.Assert().Contains(failures[0], "no such file or directory")
		h.Assert().NotContains(failures[0], constants.CgroupTalosContainersRoot)

		// the controller is in restart backoff; once the root exists the retry applies the limit
		host.seed(workloadPidsOtherRoot, "max\n")

		time.Sleep(time.Minute)
		synctest.Wait()

		h.Assert().Equal("261144", host.read(workloadPidsOtherRoot))
		h.Assert().Equal(1, h.rootWrites(constants.CgroupTalosContainersRoot))
		h.Assert().Len(h.failures(), 1)
	})
}

func TestWorkloadPidsAllRootsFailingAggregates(t *testing.T) {
	t.Parallel()

	host := newWorkloadPidsHost(t, "max\n", "262144", "500000")
	require.NoError(t, os.Remove(host.path(constants.CgroupTalosContainersRoot)))

	roots := []runtimectrls.WorkloadPidsRoot{
		talosContainersPidsRoot,
		{Cgroup: workloadPidsOtherRoot, ReservedPids: 1000},
	}

	runWorkloadPidsRoots(t, host, machineruntime.ModeMetal, roots, func(h *workloadPidsHarness) {
		failures := h.failures()
		h.Require().Len(failures, 1)
		h.Assert().Contains(failures[0], constants.CgroupTalosContainersRoot)
		h.Assert().Contains(failures[0], workloadPidsOtherRoot)
	})
}

func TestWorkloadPidsNoRootsIsAnError(t *testing.T) {
	t.Parallel()

	host := newWorkloadPidsHost(t, "max\n", "262144", "500000")

	runWorkloadPidsRoots(t, host, machineruntime.ModeMetal, nil, func(h *workloadPidsHarness) {
		failures := h.failures()
		h.Require().NotEmpty(failures)
		h.Assert().Contains(failures[0], "no cgroup roots configured")
		h.Assert().Equal("max\n", host.read(constants.CgroupTalosContainersRoot))
	})
}

func TestWorkloadPidsControllerDefinition(t *testing.T) {
	t.Parallel()

	ctrl := &runtimectrls.WorkloadPidsController{}

	assert.Equal(t, "runtime.WorkloadPidsController", ctrl.Name())
	assert.Nil(t, ctrl.Outputs())

	inputs := ctrl.Inputs()
	require.Len(t, inputs, 1)
	assert.Equal(t, runtime.KernelParamStatusType, inputs[0].Type)
}
