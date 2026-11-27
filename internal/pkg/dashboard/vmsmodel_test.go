// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// The VM list is a join across several watched resources, so the join and the formatting live in
// free functions and are tested here without a terminal.
//
//nolint:testpackage
package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func newVMSpec(name string, spec hypervisor.VirtualMachineSpecSpec) *hypervisor.VirtualMachineSpec {
	res := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
	*res.TypedSpec() = spec

	return res
}

func newVMStatus(name string, spec hypervisor.VirtualMachineStatusSpec) *hypervisor.VirtualMachineStatus {
	res := hypervisor.NewVirtualMachineStatus(hypervisor.NamespaceName, name)
	*res.TypedSpec() = spec

	return res
}

func newVMDomain(name string, spec hypervisor.VirtualMachineDomainStatusSpec) *hypervisor.VirtualMachineDomainStatus {
	res := hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, name)
	*res.TypedSpec() = spec

	return res
}

func newVMDisk(id string, spec hypervisor.VirtualMachineDiskStatusSpec) *hypervisor.VirtualMachineDiskStatus {
	res := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, id)
	*res.TypedSpec() = spec

	return res
}

func testVMResources() *vmResources {
	res := newVMResources()

	res.specs["web"] = newVMSpec("web", hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 4, Limit: 2500},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi", SecureBoot: true},
		Console:    hypervisor.VirtualMachineConsoleSpec{Serial: true, VNC: true},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
		Disks: []hypervisor.VirtualMachineDiskSpec{
			{Name: "root", Pool: "fast", Size: 20 << 30, Type: "disk", Bus: "virtio", Provision: hypervisor.VirtualMachineDiskProvisionSpec{
				FromImage: &hypervisor.VirtualMachineDiskFromImageSpec{Library: "images", File: "alpine.qcow2"},
			}},
			{Name: "cd", Type: "cdrom", Bus: "sata", Provision: hypervisor.VirtualMachineDiskProvisionSpec{Blank: true}},
		},
		Interfaces: []hypervisor.VirtualMachineInterfaceSpec{
			{Name: "eth0", Link: "br0", Attachment: "bridge", HardwareAddr: "52:54:00:00:00:01"},
		},
	})
	res.statuses["web"] = newVMStatus("web", hypervisor.VirtualMachineStatusSpec{
		PowerState: hypervisorhelpers.VirtualMachinePowerStateRunning,
		Stage:      hypervisorhelpers.VirtualMachineStageReady,
		Addresses:  []string{"10.0.0.5/24", "fd00::5/64"},
	})
	res.domains["web"] = newVMDomain("web", hypervisor.VirtualMachineDomainStatusSpec{
		VCPUs: 4, MemoryKiB: 2 << 20, MaxMemoryKiB: 4 << 20,
	})

	res.specs["db"] = newVMSpec("db", hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 2},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 8 << 30},
		PowerState: "stopped",
	})
	res.statuses["db"] = newVMStatus("db", hypervisor.VirtualMachineStatusSpec{
		PowerState: hypervisorhelpers.VirtualMachinePowerStateStopped,
		Stage:      hypervisorhelpers.VirtualMachineStageReady,
	})

	// An unmanaged libvirt domain has a domain status and nothing else.
	res.domains["stray"] = newVMDomain("stray", hypervisor.VirtualMachineDomainStatusSpec{VCPUs: 1})

	return res
}

func TestBuildVMRows(t *testing.T) {
	res := testVMResources()

	rows := buildVMRows(res, "", vmSortName)

	require.Len(t, rows, 2, "the unmanaged domain must not be listed")

	assert.Equal(t, "db", rows[0].Name)
	assert.Equal(t, vmHealthStopped, rows[0].Health)
	assert.Equal(t, uint32(2), rows[0].VCPUs, "without a domain the configured vCPUs are shown")
	assert.False(t, rows[0].MemoryLive)

	assert.Equal(t, "web", rows[1].Name)
	assert.Equal(t, vmHealthRunning, rows[1].Health)
	assert.True(t, rows[1].MemoryLive)
	assert.Equal(t, "2.1 GB / 4.3 GB", formatVMMemory(rows[1]))
	assert.Equal(t, "10.0.0.5/24 (+1)", formatVMAddresses(rows[1].Addresses))
}

func TestBuildVMRowsSpecOnly(t *testing.T) {
	res := newVMResources()
	res.specs["new"] = newVMSpec("new", hypervisor.VirtualMachineSpecSpec{CPU: hypervisor.VirtualMachineCPUSpec{Count: 1}})

	rows := buildVMRows(res, "", vmSortName)

	require.Len(t, rows, 1)
	assert.Equal(t, vmHealthUnknown, rows[0].Health, "a VM nobody has observed yet is unknown, not stopped")
	assert.Equal(t, "unknown", rows[0].Power)
}

func TestVMOverrideMarker(t *testing.T) {
	res := testVMResources()
	res.overrides["web"] = func() *hypervisor.VirtualMachinePowerOverride {
		override := hypervisor.NewVirtualMachinePowerOverride(hypervisor.NamespaceName, "web")
		override.TypedSpec().PowerState = "stopped"

		return override
	}()

	rows := buildVMRows(res, "web", vmSortName)

	require.Len(t, rows, 1)
	assert.Equal(t, "running*", rows[0].Power)
	assert.True(t, rows[0].Overridden)

	detail := formatVMDetail(res, "web")
	assert.Contains(t, detail, "declared running")
	assert.Contains(t, detail, "overridden to stopped until the host reboots")
}

func TestVMHealth(t *testing.T) {
	for _, test := range []struct {
		name   string
		status *hypervisor.VirtualMachineStatus
		want   vmHealth
	}{
		{"none", nil, vmHealthUnknown},
		{"error stage", newVMStatus("x", hypervisor.VirtualMachineStatusSpec{Stage: hypervisorhelpers.VirtualMachineStageError}), vmHealthError},
		{"error text wins", newVMStatus("x", hypervisor.VirtualMachineStatusSpec{
			Stage: hypervisorhelpers.VirtualMachineStageReady, PowerState: hypervisorhelpers.VirtualMachinePowerStateRunning, Error: "boom",
		}), vmHealthError},
		{"pending", newVMStatus("x", hypervisor.VirtualMachineStatusSpec{Stage: hypervisorhelpers.VirtualMachineStagePending}), vmHealthPending},
		{"unobserved", newVMStatus("x", hypervisor.VirtualMachineStatusSpec{}), vmHealthUnknown},
		{"ready running", newVMStatus("x", hypervisor.VirtualMachineStatusSpec{
			Stage: hypervisorhelpers.VirtualMachineStageReady, PowerState: hypervisorhelpers.VirtualMachinePowerStateRunning,
		}), vmHealthRunning},
		{"ready stopped", newVMStatus("x", hypervisor.VirtualMachineStatusSpec{
			Stage: hypervisorhelpers.VirtualMachineStageReady, PowerState: hypervisorhelpers.VirtualMachinePowerStateStopped,
		}), vmHealthStopped},
	} {
		assert.Equal(t, test.want, vmHealthOf(test.status), test.name)
	}
}

func TestMatchesVMFilter(t *testing.T) {
	row := buildVMRows(testVMResources(), "", vmSortName)[1]

	for filter, want := range map[string]bool{
		"":      true,
		"WEB":   true,
		"run":   true,
		"ready": true,
		"10.0.": true,
		"db":    false,
	} {
		assert.Equal(t, want, matchesVMFilter(row, filter), filter)
	}
}

func TestSortVMRows(t *testing.T) {
	res := testVMResources()

	res.specs["err"] = newVMSpec("err", hypervisor.VirtualMachineSpecSpec{CPU: hypervisor.VirtualMachineCPUSpec{Count: 8}})
	res.statuses["err"] = newVMStatus("err", hypervisor.VirtualMachineStatusSpec{
		Stage: hypervisorhelpers.VirtualMachineStageError, Error: "boom",
	})

	names := func(sortBy vmSort) []string {
		result := make([]string, 0, 3)

		for _, row := range buildVMRows(res, "", sortBy) {
			result = append(result, row.Name)
		}

		return result
	}

	assert.Equal(t, []string{"db", "err", "web"}, names(vmSortName))
	assert.Equal(t, []string{"err", "web", "db"}, names(vmSortHealth), "errors first")
	assert.Equal(t, []string{"err", "web", "db"}, names(vmSortVCPUs), "most vCPUs first")
	assert.Equal(t, []string{"db", "web", "err"}, names(vmSortMemory), "largest first, unknown last")
	assert.Equal(t, vmSortName, vmSortMemory.next())
}

func TestVMSummary(t *testing.T) {
	assert.Equal(t, "no VMs", vmSummary(nil, false))
	assert.Contains(t, vmSummary(nil, true), "host shutting down")

	summary := vmSummary(buildVMRows(testVMResources(), "", vmSortName), false)

	assert.Contains(t, summary, "2 VMs")
	assert.Contains(t, summary, "1 running")
	assert.Contains(t, summary, "1 stopped")
}

func TestVMShuttingDown(t *testing.T) {
	res := newVMResources()
	assert.False(t, res.shuttingDown())

	res.lifecycle = hypervisor.NewVirtualMachineLifecycle(hypervisor.NamespaceName, hypervisor.VirtualMachineLifecycleID)
	assert.False(t, res.shuttingDown())

	res.lifecycle.TypedSpec().ShutdownDeadline = time.Now().Add(time.Minute)
	assert.True(t, res.shuttingDown())
}

func TestFormatVMError(t *testing.T) {
	assert.Equal(t, "a b", formatVMError("a\n  b"))
	assert.Equal(t, vmGracefulStopHint, formatVMError(hypervisor.GracefulStopPendingError))
	assert.Empty(t, formatVMError(""))
}

func TestFormatVMDetail(t *testing.T) {
	res := testVMResources()

	detail := formatVMDetail(res, "web")

	for _, want := range []string{"running", "4 · limit 2.5 CPU", "2.1 GB / 4.3 GB", "uefi · secure boot · profile host", "serial, vnc", "10.0.0.5/24, fd00::5/64"} {
		assert.Contains(t, detail, want)
	}

	assert.Contains(t, formatVMDetail(res, "db"), "guest agent is not enabled")
	assert.Contains(t, formatVMDetail(res, "gone"), "no longer present")
}

func TestVMDetailFitsPane(t *testing.T) {
	res := testVMResources()
	res.statuses["web"].TypedSpec().Error = hypervisor.GracefulStopPendingError

	// A pane of vmDetailRows has the two border rows, so the fields must not be more than the rest.
	lines := strings.Count(strings.TrimRight(formatVMDetail(res, "web"), "\n"), "\n") + 1

	assert.LessOrEqual(t, lines, vmDetailRows-2)
}

func TestVMDeviceRows(t *testing.T) {
	res := testVMResources()

	// Two statuses for one disk: the ready one wins whatever the IDs.
	res.disks["web/root@bbb"] = newVMDisk("web/root@bbb", hypervisor.VirtualMachineDiskStatusSpec{
		VirtualMachine: "web", Name: "root", Phase: hypervisor.VirtualMachineDiskPhaseReady, Size: 32 << 30,
	})
	res.disks["web/root@aaa"] = newVMDisk("web/root@aaa", hypervisor.VirtualMachineDiskStatusSpec{
		VirtualMachine: "web", Name: "root", Phase: hypervisor.VirtualMachineDiskPhaseNotReady, Error: "stale",
	})
	res.disks["db/root@ccc"] = newVMDisk("db/root@ccc", hypervisor.VirtualMachineDiskStatusSpec{VirtualMachine: "db", Name: "root"})

	rows := vmDeviceRows(res, "web")

	require.Len(t, rows, 3)
	assert.Equal(t, []string{"disk", "root", "disk/virtio", "32 GiB", "ready", "image images/alpine.qcow2"}, rows[0])
	assert.Equal(t, []string{"disk", "cd", "cdrom/sata", metricUnknown, "pending", "empty drive"}, rows[1])
	assert.Equal(t, []string{"nic", "eth0", "bridge", metricUnknown, metricUnknown, "link br0 · 52:54:00:00:00:01"}, rows[2])

	assert.Nil(t, vmDeviceRows(res, "gone"))
}

func TestVMLogsAvailable(t *testing.T) {
	res := testVMResources()

	assert.True(t, vmLogsAvailable(res.specs["web"], res.domains["web"]))
	assert.Empty(t, vmLogsHint(res.specs["web"], res.domains["web"]))

	assert.False(t, vmLogsAvailable(res.specs["db"], res.domains["web"]), "no serial console")
	assert.Equal(t, "serial console not configured", vmLogsHint(res.specs["db"], nil))

	assert.False(t, vmLogsAvailable(res.specs["web"], nil), "not defined in libvirt yet")
	assert.Equal(t, "waiting for the VM to be defined", vmLogsHint(res.specs["web"], nil))

	assert.False(t, vmLogsAvailable(nil, res.domains["web"]))
	assert.Equal(t, "waiting for the VM configuration", vmLogsHint(nil, res.domains["web"]))
}
