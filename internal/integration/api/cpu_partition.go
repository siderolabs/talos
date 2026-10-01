// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// CPUPartitionSuite exercises the CPU partition policy against a real libvirt host.
//
// It needs a node running the libvirtd extension (virtqemud) with at least 8 online CPUs;
// otherwise it skips. Every step asserts on the kernel (cgroupfs, /proc) rather than on
// reported status alone, and no virtual machine is ever stopped by the policy: only the
// test's explicit powerState changes stop them.
type CPUPartitionSuite struct {
	base.APISuite
}

// SuiteName implements base.NamedSuite.
func (suite *CPUPartitionSuite) SuiteName() string {
	return "api.CPUPartitionSuite"
}

const (
	cpuPartitionBusybox = "/nix/var/nix/profiles/default/bin/busybox"
	cpuPartitionVMRoot  = "/virtualmachines.partition"
)

// acceptedPartition is the accepted policy on an 8-CPU host.
func acceptedPartition() *runtimecfg.CPUPartitionConfigV1Alpha1 {
	doc := runtimecfg.NewCPUPartitionConfigV1Alpha1()
	doc.InitConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.SystemConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.PodRuntimeConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.KubepodsConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "2-3"}
	doc.TalosContainersConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.VirtualMachinesConfig = &runtimecfg.CPUPartitionVirtualMachines{
		RootCPUs:     "4-7",
		SlicesConfig: []runtimecfg.CPUPartitionSlice{{SliceName: "database", SliceCPUs: "4-5", SliceExclusive: new(true)}},
	}

	return doc
}

func cpuPartitionVM(name, slice string, running bool) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateStopped

	if running {
		doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	}

	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig = hypervisorcfg.VirtualMachineCPU{CPUCount: 2, CPUSlice: slice}
	doc.MemoryConfig = hypervisorcfg.VirtualMachineMemory{MemorySize: meta.MustByteSize("256MiB")}

	return doc
}

// domainState is the kernel-level identity of a running domain.
type domainState struct {
	pid     string
	cgroup  string
	threads map[string]string // tid -> Cpus_allowed_list
}

func (suite *CPUPartitionSuite) skipUnlessLibvirtHost(ctx context.Context, node string) {
	if testing.Short() {
		suite.T().Skip("skipping machine configuration changes in short mode")
	}

	// The policy bounds the kubelet's pod cgroup, which only a kubelet creates: without
	// Kubernetes the coordinator would wait on that barrier for good.
	if !suite.Capabilities().SupportsKubernetes {
		suite.T().Skip("cluster doesn't run Kubernetes, so the kubepods cgroup the policy bounds never appears")
	}

	nodeCtx := client.WithNode(ctx, node)

	resp, err := suite.Client.ServiceInfo(nodeCtx, "ext-virtqemud")
	if err != nil || len(resp) == 0 || resp[0].Service.State != "Running" {
		suite.T().Skip("virtqemud is not running on the node")
	}

	online, err := cpuset.Parse(suite.ReadFile(nodeCtx, "/sys/devices/system/cpu/online"))
	suite.Require().NoError(err)

	if online.Size() < 8 {
		suite.T().Skipf("node has %d online CPUs, the partition fixture needs 8", online.Size())
	}
}

func (suite *CPUPartitionSuite) effective(nodeCtx context.Context, cgroup string) string {
	return suite.ReadFile(nodeCtx, "/sys/fs/cgroup"+cgroup+"/cpuset.cpus.effective")
}

func (suite *CPUPartitionSuite) configured(nodeCtx context.Context, cgroup string) string {
	return suite.ReadFile(nodeCtx, "/sys/fs/cgroup"+cgroup+"/cpuset.cpus")
}

// inVMRoot reports whether a /proc/<pid>/cgroup line places the domain directly in the virtual
// machine root (its libvirt machine cgroup is a direct child), not in a slice partition.
func inVMRoot(cgroupLine string) bool {
	_, path, ok := strings.Cut(cgroupLine, "::")
	if !ok {
		return false
	}

	rest, ok := strings.CutPrefix(path, cpuPartitionVMRoot+"/")

	return ok && !strings.Contains(strings.SplitN(rest, "/", 2)[0], ".partition")
}

// exists reports whether the cgroup directory exists on the node.
func (suite *CPUPartitionSuite) exists(nodeCtx context.Context, cgroup string) bool {
	reader, err := suite.Client.Read(nodeCtx, "/sys/fs/cgroup"+cgroup+"/cgroup.events")
	if err != nil {
		return false
	}

	defer reader.Close() //nolint:errcheck

	// The stream reports a missing file while reading, not on open.
	_, err = io.Copy(io.Discard, reader)

	return err == nil
}

// domain reads the QEMU host PID from libvirt's pidfile and every thread's affinity from /proc.
func (suite *CPUPartitionSuite) domain(ctx context.Context, node, name string) domainState {
	script := fmt.Sprintf(`B=%s
pid=$($B cat /run/libvirt/qemu/%s.pid 2>/dev/null) || exit 3
printf 'PID %%s\n' "$pid"
printf 'CGROUP %%s\n' "$($B cat /proc/$pid/cgroup)"
for t in /proc/$pid/task/*; do
  printf 'THREAD %%s %%s\n' "${t##*/}" "$($B grep Cpus_allowed_list "$t/status" | $B cut -f2)"
done`, cpuPartitionBusybox, name)

	out, code := suite.RunDebugContainer(ctx, node, "sh", "-c", script)
	suite.Require().Zero(code, "domain %s is not running: %s", name, out)

	state := domainState{threads: map[string]string{}}

	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch fields[0] {
		case "PID":
			state.pid = fields[1]
		case "CGROUP":
			state.cgroup = fields[1]
		case "THREAD":
			if len(fields) == 3 {
				state.threads[fields[1]] = fields[2]
			}
		}
	}

	suite.Require().NotEmpty(state.pid)
	suite.Require().NotEmpty(state.threads)

	return state
}

func (suite *CPUPartitionSuite) domainAbsent(ctx context.Context, node, name string) bool {
	_, code := suite.RunDebugContainer(ctx, node, "sh", "-c", fmt.Sprintf("%s test ! -f /run/libvirt/qemu/%s.pid", cpuPartitionBusybox, name))

	return code == 0
}

// assertThreadsWithin asserts every QEMU thread (vCPUs and emulator) is bounded by cpus.
func (suite *CPUPartitionSuite) assertThreadsWithin(state domainState, cpus string) {
	bound, err := cpuset.Parse(cpus)
	suite.Require().NoError(err)

	for tid, mask := range state.threads {
		set, err := cpuset.Parse(mask)
		suite.Require().NoError(err)
		suite.Assert().True(set.IsSubsetOf(bound), "thread %s of pid %s allowed %q, expected within %q", tid, state.pid, mask, cpus)
	}
}

func (suite *CPUPartitionSuite) apply(nodeCtx context.Context, base config.Provider, docs ...configcfg.Document) {
	suite.T().Helper()

	all := append(slices.Clone(base.Documents()), docs...)

	cfg, err := container.New(all...)
	suite.Require().NoError(err)

	data, err := cfg.Bytes()
	suite.Require().NoError(err)

	_, err = suite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{Data: data, Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT})
	suite.Require().NoError(err)
}

func (suite *CPUPartitionSuite) status(nodeCtx context.Context) *runtimeres.CPUPartitionStatusSpec {
	status, err := safe.StateGetByID[*runtimeres.CPUPartitionStatus](nodeCtx, suite.Client.COSI, runtimeres.CPUPartitionStatusID)
	if err != nil {
		return nil
	}

	return status.TypedSpec()
}

func (suite *CPUPartitionSuite) waitPhase(nodeCtx context.Context, phase runtimeres.CPUPartitionPhase) *runtimeres.CPUPartitionStatusSpec {
	suite.T().Helper()

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, runtimeres.CPUPartitionStatusID,
		func(status *runtimeres.CPUPartitionStatus, asrt *assert.Assertions) {
			asrt.Equal(phase, status.TypedSpec().Phase, "%+v", *status.TypedSpec())
		})

	return suite.status(nodeCtx)
}

func (suite *CPUPartitionSuite) waitDomainAbsent(ctx context.Context, node, name string) {
	suite.T().Helper()

	suite.Require().Eventually(func() bool { return suite.domainAbsent(ctx, node, name) }, 2*time.Minute, 2*time.Second, "domain %s still running", name)
}

// TestPolicyLifecycle runs the acceptance sequence: unpartitioned baseline, blocked introduction
// over a running root machine, operator stop and activation, boundary verification, live shrink
// with domain identity preserved, blocked fully occupied swap, operator-driven swap, blocked
// removal and restoration.
//
//nolint:gocyclo,cyclop,maintidx
func (suite *CPUPartitionSuite) TestPolicyLifecycle() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(ctx, node)

	suite.skipUnlessLibvirtHost(ctx, node)

	original, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)
	suite.Require().Nil(original.CPUPartitionConfig(), "the node already carries a CPU partition policy")

	originalBytes, err := original.Bytes()
	suite.Require().NoError(err)

	suite.T().Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()

		cleanupCtx = client.WithNode(cleanupCtx, node)

		// Stop the machines first so the policy can be restored, then drop everything.
		suite.apply(cleanupCtx, original, acceptedPartition(), cpuPartitionVM("cpupart-db", "database", false), cpuPartitionVM("cpupart-web", "", false))
		suite.waitDomainAbsent(cleanupCtx, node, "cpupart-db")
		suite.waitDomainAbsent(cleanupCtx, node, "cpupart-web")

		_, applyErr := suite.Client.ApplyConfiguration(cleanupCtx, &machineapi.ApplyConfigurationRequest{Data: originalBytes, Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT})
		suite.Assert().NoError(applyErr)

		rtestutils.AssertNoResource[*runtimeres.CPUPartitionStatus](cleanupCtx, suite.T(), suite.Client.COSI, runtimeres.CPUPartitionStatusID)
	})

	// 1. Baseline: no policy, an ordinary machine runs in the virtual machine root.
	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, runtimeres.CPUPartitionSpecID,
		func(spec *runtimeres.CPUPartitionSpec, asrt *assert.Assertions) { asrt.False(spec.TypedSpec().Enabled) })
	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, k8s.KubeletID,
		func(res *k8s.KubeletCPUReservation, asrt *assert.Assertions) { asrt.False(res.TypedSpec().Managed) })

	suite.apply(nodeCtx, original, cpuPartitionVM("cpupart-web", "", true))

	var web domainState

	suite.Require().Eventually(func() bool {
		return !suite.domainAbsent(ctx, node, "cpupart-web")
	}, 2*time.Minute, 2*time.Second)

	web = suite.domain(ctx, node, "cpupart-web")
	suite.Require().True(inVMRoot(web.cgroup), "baseline machine runs directly in the root: %s", web.cgroup)
	suite.T().Logf("baseline web pid=%s cgroup=%s", web.pid, web.cgroup)

	domainXML := func(name string) string {
		spec, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](nodeCtx, suite.Client.COSI, name)
		suite.Require().NoError(err)

		return spec.TypedSpec().DomainXML
	}

	webXML := domainXML("cpupart-web")
	initMask := suite.configured(nodeCtx, "/init")

	// Introducing the policy while web runs in the root is blocked: nothing written, no restart.
	suite.apply(nodeCtx, original, acceptedPartition(), cpuPartitionVM("cpupart-web", "", true), cpuPartitionVM("cpupart-db", "database", true))

	blocked := suite.waitPhase(nodeCtx, runtimeres.CPUPartitionPhaseBlocked)
	suite.Require().NotEmpty(blocked.Blocked)
	suite.Assert().Contains(blocked.Blocked[0].VirtualMachines, "cpupart-web")
	suite.Assert().Empty(blocked.Targets, "no mask applied while blocked")
	suite.Assert().Equal(initMask, suite.configured(nodeCtx, "/init"), "init mask untouched")
	suite.Assert().Equal(webXML, domainXML("cpupart-web"), "the running definition is unchanged")

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, k8s.KubeletID,
		func(res *k8s.KubeletCPUReservation, asrt *assert.Assertions) { asrt.False(res.TypedSpec().Managed) })

	again := suite.domain(ctx, node, "cpupart-web")
	suite.Assert().Equal(web.pid, again.pid, "web must not be restarted by a blocked introduction")
	suite.Assert().True(suite.domainAbsent(ctx, node, "cpupart-db"), "db must not start under a blocked policy")

	// Operator stops web; actual release lets the policy activate; both start in their partitions.
	suite.apply(nodeCtx, original, acceptedPartition(), cpuPartitionVM("cpupart-web", "", false), cpuPartitionVM("cpupart-db", "database", true))
	suite.waitDomainAbsent(ctx, node, "cpupart-web")
	suite.waitPhase(nodeCtx, runtimeres.CPUPartitionPhaseReady)

	suite.apply(nodeCtx, original, acceptedPartition(), cpuPartitionVM("cpupart-web", "", true), cpuPartitionVM("cpupart-db", "database", true))

	suite.Require().Eventually(func() bool {
		return !suite.domainAbsent(ctx, node, "cpupart-web") && !suite.domainAbsent(ctx, node, "cpupart-db")
	}, 3*time.Minute, 2*time.Second)

	// 2. Boundaries.
	for cgroup, want := range map[string]string{
		"/init": "0-1", "/system": "0-1", "/podruntime": "0-1", "/kubepods": "2-3", "/taloscontainers": "0-1",
		cpuPartitionVMRoot: "4-7", cpuPartitionVMRoot + "/shared.partition": "6-7", cpuPartitionVMRoot + "/database.partition": "4-5",
	} {
		suite.Assert().Equal(want, suite.effective(nodeCtx, cgroup), "effective %s", cgroup)
	}

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, k8s.KubeletID,
		func(res *k8s.KubeletCPUReservation, asrt *assert.Assertions) {
			asrt.True(res.TypedSpec().Managed)
			asrt.Equal("0-1,4-7", res.TypedSpec().ReservedCPUs)
		})
	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, k8s.KubeletID,
		func(res *k8s.KubeletSpec, asrt *assert.Assertions) {
			asrt.Equal("0-1,4-7", res.TypedSpec().Config["reservedSystemCPUs"])
		})

	suite.Assert().Contains(domainXML("cpupart-db"), "<partition>"+cpuPartitionVMRoot+"/database.partition</partition>")
	suite.Assert().Contains(domainXML("cpupart-web"), "<partition>"+cpuPartitionVMRoot+"/shared.partition</partition>")

	db := suite.domain(ctx, node, "cpupart-db")
	web = suite.domain(ctx, node, "cpupart-web")
	suite.Require().Contains(db.cgroup, "/database.partition/")
	suite.Require().Contains(web.cgroup, "/shared.partition/")
	suite.assertThreadsWithin(db, "4-5")
	suite.assertThreadsWithin(web, "6-7")
	suite.T().Logf("db pid=%s threads=%v; web pid=%s threads=%v", db.pid, db.threads, web.pid, web.threads)

	// 3. Live shrink of the occupied exclusive slice: same PID, tighter masks, no restart.
	shrunk := acceptedPartition()
	shrunk.VirtualMachinesConfig.SlicesConfig[0].SliceCPUs = "4"
	suite.apply(nodeCtx, original, shrunk, cpuPartitionVM("cpupart-web", "", true), cpuPartitionVM("cpupart-db", "database", true))
	suite.waitPhase(nodeCtx, runtimeres.CPUPartitionPhaseReady)

	suite.Require().Eventually(func() bool { return suite.effective(nodeCtx, cpuPartitionVMRoot+"/database.partition") == "4" }, time.Minute, time.Second)
	suite.Assert().Equal("5-7", suite.effective(nodeCtx, cpuPartitionVMRoot+"/shared.partition"))

	dbShrunk := suite.domain(ctx, node, "cpupart-db")
	suite.Assert().Equal(db.pid, dbShrunk.pid, "live shrink must not restart the domain")
	suite.assertThreadsWithin(dbShrunk, "4")

	// Regrow back to the accepted sets (shared gives 5 back first): still the same PID.
	suite.apply(nodeCtx, original, acceptedPartition(), cpuPartitionVM("cpupart-web", "", true), cpuPartitionVM("cpupart-db", "database", true))
	suite.waitPhase(nodeCtx, runtimeres.CPUPartitionPhaseReady)
	suite.Require().Eventually(func() bool { return suite.effective(nodeCtx, cpuPartitionVMRoot+"/database.partition") == "4-5" }, time.Minute, time.Second)
	suite.Assert().Equal(db.pid, suite.domain(ctx, node, "cpupart-db").pid)

	// 4. Fully occupied swap: blocked before anything changes.
	swap := acceptedPartition()
	swap.VirtualMachinesConfig.SlicesConfig[0].SliceCPUs = "6-7"
	suite.apply(nodeCtx, original, swap, cpuPartitionVM("cpupart-web", "", true), cpuPartitionVM("cpupart-db", "database", true))

	blocked = suite.waitPhase(nodeCtx, runtimeres.CPUPartitionPhaseBlocked)
	suite.Require().NotEmpty(blocked.Blocked)
	suite.T().Logf("swap blocked: %+v", blocked.Blocked)
	suite.Assert().Equal("4-5", suite.effective(nodeCtx, cpuPartitionVMRoot+"/database.partition"))
	suite.Assert().Equal("6-7", suite.effective(nodeCtx, cpuPartitionVMRoot+"/shared.partition"))
	suite.Assert().Equal(db.pid, suite.domain(ctx, node, "cpupart-db").pid)
	suite.Assert().Equal(web.pid, suite.domain(ctx, node, "cpupart-web").pid)

	// Operator stops both, waits for actual release, then the swap applies and both restart placed.
	suite.apply(nodeCtx, original, swap, cpuPartitionVM("cpupart-web", "", false), cpuPartitionVM("cpupart-db", "database", false))
	suite.waitDomainAbsent(ctx, node, "cpupart-web")
	suite.waitDomainAbsent(ctx, node, "cpupart-db")
	suite.waitPhase(nodeCtx, runtimeres.CPUPartitionPhaseReady)
	suite.Assert().Equal("6-7", suite.effective(nodeCtx, cpuPartitionVMRoot+"/database.partition"))
	suite.Assert().Equal("4-5", suite.effective(nodeCtx, cpuPartitionVMRoot+"/shared.partition"))

	suite.apply(nodeCtx, original, swap, cpuPartitionVM("cpupart-web", "", true), cpuPartitionVM("cpupart-db", "database", true))
	suite.Require().Eventually(func() bool {
		return !suite.domainAbsent(ctx, node, "cpupart-web") && !suite.domainAbsent(ctx, node, "cpupart-db")
	}, 3*time.Minute, 2*time.Second)
	suite.assertThreadsWithin(suite.domain(ctx, node, "cpupart-db"), "6-7")
	suite.assertThreadsWithin(suite.domain(ctx, node, "cpupart-web"), "4-5")

	// 5b. Enforcement loss: a mask changed outside Talos is reported, blocks new starts and is
	// never corrected or acted on automatically; running machines are left alone. Web is stopped
	// first so its restart is the new start under test.
	suite.apply(nodeCtx, original, swap, cpuPartitionVM("cpupart-web", "", false), cpuPartitionVM("cpupart-db", "database", true))
	suite.waitDomainAbsent(ctx, node, "cpupart-web")
	suite.waitPhase(nodeCtx, runtimeres.CPUPartitionPhaseReady)

	dbAfterSwap := suite.domain(ctx, node, "cpupart-db")
	foreignWrite := func(cpus string) {
		out, code := suite.RunDebugContainer(ctx, node, "sh", "-c",
			fmt.Sprintf("echo %s > /sys/fs/cgroup%s/database.partition/cpuset.cpus", cpus, cpuPartitionVMRoot))
		suite.Require().Zero(code, out)
	}

	foreignWrite("7")
	suite.apply(nodeCtx, original, swap, cpuPartitionVM("cpupart-web", "", true), cpuPartitionVM("cpupart-db", "database", true))

	blocked = suite.waitPhase(nodeCtx, runtimeres.CPUPartitionPhaseBlocked)
	suite.Assert().Equal([]string{"virtualMachines/database"}, blocked.EnforcementLoss)
	suite.Require().NotEmpty(blocked.Blocked)
	suite.Assert().Contains(blocked.Blocked[0].Reason, "enforcement lost")
	suite.Assert().Equal("7", suite.configured(nodeCtx, cpuPartitionVMRoot+"/database.partition"), "the foreign mask is reported, not overwritten")
	suite.Assert().Equal(dbAfterSwap.pid, suite.domain(ctx, node, "cpupart-db").pid, "db is left running")
	suite.Assert().True(suite.domainAbsent(ctx, node, "cpupart-web"), "no new start while a boundary lost enforcement")

	// The operator restores the mask; the next observation clears the loss and web starts.
	foreignWrite("6-7")
	suite.apply(nodeCtx, original, swap, cpuPartitionVM("cpupart-web", "", false), cpuPartitionVM("cpupart-db", "database", true))
	suite.Assert().Empty(suite.waitPhase(nodeCtx, runtimeres.CPUPartitionPhaseReady).EnforcementLoss)

	suite.apply(nodeCtx, original, swap, cpuPartitionVM("cpupart-web", "", true), cpuPartitionVM("cpupart-db", "database", true))
	suite.Require().Eventually(func() bool { return !suite.domainAbsent(ctx, node, "cpupart-web") }, 3*time.Minute, 2*time.Second)
	suite.assertThreadsWithin(suite.domain(ctx, node, "cpupart-web"), "4-5")
	suite.Assert().Equal(dbAfterSwap.pid, suite.domain(ctx, node, "cpupart-db").pid)

	// 6. Removing the policy while machines run is blocked; after the operator stops them the
	// owned masks are restored and the guard goes away. A machine still naming a slice is rejected
	// by validation together with the removal, so the slice reference goes away with the policy.
	suite.apply(nodeCtx, original, cpuPartitionVM("cpupart-web", "", true), cpuPartitionVM("cpupart-db", "", true))
	blocked = suite.waitPhase(nodeCtx, runtimeres.CPUPartitionPhaseBlocked)
	suite.Require().NotEmpty(blocked.Blocked)
	suite.Assert().Equal(dbAfterSwap.pid, suite.domain(ctx, node, "cpupart-db").pid)
	suite.Assert().Equal("6-7", suite.effective(nodeCtx, cpuPartitionVMRoot+"/database.partition"))

	suite.apply(nodeCtx, original, cpuPartitionVM("cpupart-web", "", false), cpuPartitionVM("cpupart-db", "", false))
	suite.waitDomainAbsent(ctx, node, "cpupart-web")
	suite.waitDomainAbsent(ctx, node, "cpupart-db")

	rtestutils.AssertNoResource[*runtimeres.CPUPartitionStatus](nodeCtx, suite.T(), suite.Client.COSI, runtimeres.CPUPartitionStatusID)

	// A populated root whose initial mask was inherited (empty) cannot be emptied again: cgroup v2
	// refuses with ENOSPC (observed live), so restoration writes the parent's effective set and the
	// root is bounded exactly as before the policy. Unpopulated roots go back to inheriting.
	online := suite.ReadFile(nodeCtx, "/sys/devices/system/cpu/online")

	for _, root := range []string{"/init", "/system", "/podruntime", "/kubepods"} {
		suite.Assert().Equal(online, suite.configured(nodeCtx, root), "populated %s restored to its parent's effective set", root)
		suite.Assert().Equal(online, suite.effective(nodeCtx, root), "effective %s", root)
	}

	suite.Assert().Equal("", suite.configured(nodeCtx, cpuPartitionVMRoot), "the empty virtual machine root inherits again")
	suite.Assert().Equal(online, suite.effective(nodeCtx, cpuPartitionVMRoot))
	suite.Assert().False(suite.exists(nodeCtx, cpuPartitionVMRoot+"/shared.partition"), "shared slice removed")
	suite.Assert().False(suite.exists(nodeCtx, cpuPartitionVMRoot+"/database.partition"), "database slice removed")

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, k8s.KubeletID,
		func(res *k8s.KubeletCPUReservation, asrt *assert.Assertions) { asrt.False(res.TypedSpec().Managed) })

	// A plain machine works again in the root.
	suite.apply(nodeCtx, original, cpuPartitionVM("cpupart-web", "", true), cpuPartitionVM("cpupart-db", "", false))
	suite.Require().Eventually(func() bool { return !suite.domainAbsent(ctx, node, "cpupart-web") }, 3*time.Minute, 2*time.Second)
	suite.Require().True(inVMRoot(suite.domain(ctx, node, "cpupart-web").cgroup))
}

func init() {
	allSuites = append(allSuites, new(CPUPartitionSuite))
}
