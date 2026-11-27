// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/rivo/tview"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// vmSort is the ordering applied to the VM list.
type vmSort int

// VM list orderings, cycled through by the sort key.
const (
	vmSortName vmSort = iota
	vmSortHealth
	vmSortVCPUs
	vmSortMemory
)

// next returns the ordering the sort key switches to.
func (sort vmSort) next() vmSort {
	if sort == vmSortMemory {
		return vmSortName
	}

	return sort + 1
}

// String implements fmt.Stringer.
func (sort vmSort) String() string {
	switch sort {
	case vmSortName:
		return "name"
	case vmSortHealth:
		return "health"
	case vmSortVCPUs:
		return "vcpus"
	case vmSortMemory:
		return "memory"
	default:
		return "name"
	}
}

// vmHealth is how much a VM wants an operator's attention, derived from its power state and stage.
type vmHealth int

// VM health values, in the order of severity used by the health sort.
const (
	vmHealthError vmHealth = iota
	vmHealthPending
	vmHealthRunning
	vmHealthStopped
	vmHealthUnknown
)

// String implements fmt.Stringer.
func (health vmHealth) String() string {
	switch health {
	case vmHealthError:
		return "error"
	case vmHealthPending:
		return "pending"
	case vmHealthRunning:
		return "running"
	case vmHealthStopped:
		return "stopped"
	case vmHealthUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// color returns the tview color tag of the health value: ANSI-16 names only, see healthColor.
func (health vmHealth) color() string {
	switch health {
	case vmHealthError:
		return "red"
	case vmHealthPending:
		return "yellow"
	case vmHealthRunning:
		return "green"
	case vmHealthStopped:
		return "gray"
	case vmHealthUnknown:
		return "white"
	default:
		return "white"
	}
}

// vmGracefulStopHint replaces the long graceful stop message, which has an actionable way out.
const vmGracefulStopHint = "graceful stop pending: the guest has not powered off; stop it again with force " +
	"(talosctl hypervisor stop --graceful=false)"

// vmResources is everything the VMs screen watches, keyed by resource ID.
type vmResources struct {
	specs     map[string]*hypervisor.VirtualMachineSpec
	statuses  map[string]*hypervisor.VirtualMachineStatus
	domains   map[string]*hypervisor.VirtualMachineDomainStatus
	disks     map[string]*hypervisor.VirtualMachineDiskStatus
	overrides map[string]*hypervisor.VirtualMachinePowerOverride

	lifecycle *hypervisor.VirtualMachineLifecycle
}

func newVMResources() *vmResources {
	return &vmResources{
		specs:     map[string]*hypervisor.VirtualMachineSpec{},
		statuses:  map[string]*hypervisor.VirtualMachineStatus{},
		domains:   map[string]*hypervisor.VirtualMachineDomainStatus{},
		disks:     map[string]*hypervisor.VirtualMachineDiskStatus{},
		overrides: map[string]*hypervisor.VirtualMachinePowerOverride{},
	}
}

// shuttingDown reports whether the host has started stopping its virtual machines.
func (res *vmResources) shuttingDown() bool {
	return res.lifecycle != nil && !res.lifecycle.TypedSpec().ShutdownDeadline.IsZero()
}

// diskStatuses returns the statuses of the disks of a VM, by disk name.
//
// A changed disk can briefly have two statuses: the ready one wins, then the lowest ID.
func (res *vmResources) diskStatuses(vm string) map[string]*hypervisor.VirtualMachineDiskStatus {
	ids := make([]string, 0, len(res.disks))

	for id, disk := range res.disks {
		if disk.TypedSpec().VirtualMachine == vm {
			ids = append(ids, id)
		}
	}

	slices.Sort(ids)

	result := map[string]*hypervisor.VirtualMachineDiskStatus{}

	for _, id := range ids {
		disk := res.disks[id]
		name := disk.TypedSpec().Name

		if existing, ok := result[name]; ok && existing.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseReady {
			continue
		}

		result[name] = disk
	}

	return result
}

// vmRow is a single VM flattened into what the list displays, joined from the spec, status and
// domain, any of which can be missing.
type vmRow struct {
	Name string

	Health vmHealth
	// Power is the power state as displayed, with the override marker.
	Power string
	Stage hypervisorhelpers.VirtualMachineStage
	Error string

	// MemoryKnown says whether the memory numbers mean anything, and MemoryLive whether they were
	// observed in libvirt rather than taken from the configuration.
	VCPUs        uint32
	MemoryBytes  uint64
	MaxMemory    uint64
	MemoryKnown  bool
	MemoryLive   bool
	Addresses    []string
	Overridden   bool
	LogsPossible bool
}

// buildVMRows joins the watched resources into filtered, sorted rows.
//
// Domain observations also cover libvirt domains nobody configured, so only VMs with a spec or a
// status are listed.
func buildVMRows(res *vmResources, filter string, sortBy vmSort) []vmRow {
	names := map[string]struct{}{}

	for name := range res.specs {
		names[name] = struct{}{}
	}

	for name := range res.statuses {
		names[name] = struct{}{}
	}

	rows := make([]vmRow, 0, len(names))

	for name := range names {
		row := buildVMRow(res, name)

		if matchesVMFilter(row, filter) {
			rows = append(rows, row)
		}
	}

	sortVMRows(rows, sortBy)

	return rows
}

func buildVMRow(res *vmResources, name string) vmRow {
	spec, status, domain := res.specs[name], res.statuses[name], res.domains[name]
	_, overridden := res.overrides[name]

	row := vmRow{
		Name:       name,
		Overridden: overridden,
		Power:      hypervisorhelpers.VirtualMachinePowerStateUnknown.String(),
		Stage:      hypervisorhelpers.VirtualMachineStageUnknown,
	}

	if status != nil {
		row.Power = status.TypedSpec().PowerState.String()
		row.Stage = status.TypedSpec().Stage
		row.Error = status.TypedSpec().Error
		row.Addresses = status.TypedSpec().Addresses
	}

	if overridden {
		row.Power += "*"
	}

	switch {
	case domain != nil:
		row.VCPUs = domain.TypedSpec().VCPUs
		row.MemoryBytes = domain.TypedSpec().MemoryKiB * humanize.KiByte
		row.MaxMemory = domain.TypedSpec().MaxMemoryKiB * humanize.KiByte
		row.MemoryKnown, row.MemoryLive = true, true
	case spec != nil:
		row.VCPUs = spec.TypedSpec().CPU.Count
		row.MaxMemory = spec.TypedSpec().Memory.Size
		row.MemoryKnown = true
	}

	row.Health = vmHealthOf(status)
	row.LogsPossible = vmLogsAvailable(spec, domain)

	return row
}

// vmHealthOf derives the health from the status; a VM without one has not been observed yet.
func vmHealthOf(status *hypervisor.VirtualMachineStatus) vmHealth {
	if status == nil {
		return vmHealthUnknown
	}

	spec := status.TypedSpec()

	switch {
	case spec.Stage == hypervisorhelpers.VirtualMachineStageError || spec.Error != "":
		return vmHealthError
	case spec.Stage == hypervisorhelpers.VirtualMachineStagePending:
		return vmHealthPending
	case spec.Stage != hypervisorhelpers.VirtualMachineStageReady:
		return vmHealthUnknown
	case spec.PowerState == hypervisorhelpers.VirtualMachinePowerStateRunning:
		return vmHealthRunning
	case spec.PowerState == hypervisorhelpers.VirtualMachinePowerStateStopped:
		return vmHealthStopped
	default:
		return vmHealthUnknown
	}
}

// vmLogsAvailable reports whether there is a serial log to ask for.
//
// libvirt writes it once the domain exists and keeps it afterwards, so a stopped VM still has one.
func vmLogsAvailable(spec *hypervisor.VirtualMachineSpec, domain *hypervisor.VirtualMachineDomainStatus) bool {
	return spec != nil && spec.TypedSpec().Console.Serial && domain != nil
}

// vmLogsHint explains why there are no logs, or returns an empty string when there are.
func vmLogsHint(spec *hypervisor.VirtualMachineSpec, domain *hypervisor.VirtualMachineDomainStatus) string {
	switch {
	case spec == nil:
		return "waiting for the VM configuration"
	case !spec.TypedSpec().Console.Serial:
		return "serial console not configured"
	case domain == nil:
		return "waiting for the VM to be defined"
	default:
		return ""
	}
}

// matchesVMFilter reports whether the row matches the list filter, which is a case-insensitive
// substring match against the name, power, stage and addresses.
func matchesVMFilter(row vmRow, filter string) bool {
	if filter == "" {
		return true
	}

	filter = strings.ToLower(filter)

	for _, candidate := range append([]string{row.Name, row.Power, row.Stage.String()}, row.Addresses...) {
		if strings.Contains(strings.ToLower(candidate), filter) {
			return true
		}
	}

	return false
}

// sortVMRows orders rows in place, falling back to the name so that ties do not reshuffle.
func sortVMRows(rows []vmRow, sortBy vmSort) {
	slices.SortFunc(rows, func(a, b vmRow) int {
		var primary int

		switch sortBy {
		case vmSortName:
		case vmSortHealth:
			primary = cmp.Compare(a.Health, b.Health)
		case vmSortVCPUs:
			primary = compareMetricDesc(vcpuMetric(a), vcpuMetric(b))
		case vmSortMemory:
			primary = compareMetricDesc(vmMemoryMetric(a), vmMemoryMetric(b))
		}

		if primary != 0 {
			return primary
		}

		return cmp.Compare(a.Name, b.Name)
	})
}

func vcpuMetric(row vmRow) float64 {
	if row.VCPUs == 0 {
		return math.NaN()
	}

	return float64(row.VCPUs)
}

func vmMemoryMetric(row vmRow) float64 {
	if !row.MemoryKnown {
		return math.NaN()
	}

	return float64(row.MaxMemory)
}

// formatVMVCPUs renders the vCPU count, or a placeholder when unknown.
func formatVMVCPUs(row vmRow) string {
	if row.VCPUs == 0 {
		return metricUnknown
	}

	return fmt.Sprint(row.VCPUs)
}

// formatVMMemory renders the memory in use out of the maximum when libvirt reports both, and the
// configured size otherwise.
func formatVMMemory(row vmRow) string {
	switch {
	case !row.MemoryKnown:
		return metricUnknown
	case row.MemoryLive:
		return humanize.Bytes(row.MemoryBytes) + " / " + humanize.Bytes(row.MaxMemory)
	default:
		return humanize.Bytes(row.MaxMemory)
	}
}

// formatVMAddresses renders the first guest address and how many more there are.
func formatVMAddresses(addresses []string) string {
	switch len(addresses) {
	case 0:
		return metricUnknown
	case 1:
		return addresses[0]
	default:
		return fmt.Sprintf("%s (+%d)", addresses[0], len(addresses)-1)
	}
}

// formatVMError renders an error on one line, replacing the long graceful stop message with a
// short one that still names the way out.
func formatVMError(errText string) string {
	if errText == hypervisor.GracefulStopPendingError {
		return vmGracefulStopHint
	}

	return strings.Join(strings.Fields(errText), " ")
}

// vmSummary renders the counts by health shown under the list.
func vmSummary(rows []vmRow, shuttingDown bool) string {
	parts := []string{"no VMs"}

	if len(rows) > 0 {
		counts := map[vmHealth]int{}
		for _, row := range rows {
			counts[row.Health]++
		}

		parts = []string{fmt.Sprintf("%d VMs", len(rows))}

		for _, health := range []vmHealth{vmHealthRunning, vmHealthStopped, vmHealthPending, vmHealthError, vmHealthUnknown} {
			if count := counts[health]; count > 0 {
				parts = append(parts, colored(health.color(), fmt.Sprintf("%d %s", count, health)))
			}
		}
	}

	if shuttingDown {
		parts = append(parts, colored("yellow", "host shutting down"))
	}

	return strings.Join(parts, " · ")
}

// vmDetailRows is the height of the top detail pane: eight field lines plus two border rows.
const vmDetailRows = 10

// formatVMDetail renders the field summary of one VM. Any of the resources may be nil.
func formatVMDetail(res *vmResources, name string) string {
	spec, status, domain := res.specs[name], res.statuses[name], res.domains[name]

	if spec == nil && status == nil && domain == nil {
		return "[gray]This VM is no longer present.[-]"
	}

	row := buildVMRow(res, name)

	fields := [][2]string{
		{"Power", formatVMPower(res, name, spec, row)},
		{"Stage", colored(row.Health.color(), tview.Escape(row.Stage.String()))},
	}

	if row.Error != "" {
		fields = append(fields, [2]string{"Error", colored("red", tview.Escape(formatVMError(row.Error)))})
	}

	fields = append(fields,
		[2]string{"vCPUs", formatVMCPU(spec, row)},
		[2]string{"Memory", formatVMMemory(row)},
	)

	if spec != nil {
		fields = append(fields,
			[2]string{"Firmware", formatVMFirmware(spec.TypedSpec())},
			[2]string{"Consoles", formatVMConsoles(spec.TypedSpec())},
		)
	}

	fields = append(fields, [2]string{"Addresses", formatVMGuestAddresses(spec, status, row)})

	return renderFields(fields)
}

// formatVMPower renders the observed power state together with the declared one and any override.
func formatVMPower(res *vmResources, name string, spec *hypervisor.VirtualMachineSpec, row vmRow) string {
	text := colored(row.Health.color(), strings.TrimSuffix(row.Power, "*"))

	var notes []string

	if spec != nil {
		notes = append(notes, "declared "+spec.TypedSpec().PowerState)
	}

	if override, ok := res.overrides[name]; ok {
		notes = append(notes, fmt.Sprintf("overridden to %s until the host reboots", override.TypedSpec().PowerState))
	}

	if len(notes) == 0 {
		return text
	}

	return text + " (" + strings.Join(notes, ", ") + ")"
}

func formatVMCPU(spec *hypervisor.VirtualMachineSpec, row vmRow) string {
	text := formatVMVCPUs(row)

	if spec == nil {
		return text
	}

	cpu := spec.TypedSpec().CPU

	if cpu.Limit > 0 {
		text += fmt.Sprintf(" · limit %.1f CPU", float64(cpu.Limit)/1000)
	}

	if len(cpu.Pins) > 0 || cpu.EmulatorPin != "" {
		text += " · pinned"
	}

	return text
}

func formatVMFirmware(spec *hypervisor.VirtualMachineSpecSpec) string {
	parts := []string{spec.Firmware.Type}

	if spec.Firmware.SecureBoot {
		parts = append(parts, "secure boot")
	}

	profile := spec.Profile
	if profile == "" {
		profile = "host"
	}

	parts = append(parts, "profile "+profile)

	return strings.Join(parts, " · ")
}

func formatVMConsoles(spec *hypervisor.VirtualMachineSpecSpec) string {
	var consoles []string

	if spec.Console.Serial {
		consoles = append(consoles, "serial")
	}

	if spec.Console.VNC {
		consoles = append(consoles, "vnc")
	}

	if len(consoles) == 0 {
		return none
	}

	return strings.Join(consoles, ", ")
}

// formatVMGuestAddresses renders the guest addresses, or why there are none.
func formatVMGuestAddresses(spec *hypervisor.VirtualMachineSpec, status *hypervisor.VirtualMachineStatus, row vmRow) string {
	switch {
	case status != nil && len(row.Addresses) > 0:
		return strings.Join(row.Addresses, ", ")
	case spec != nil && !spec.TypedSpec().Guest.Agent.Enabled:
		return "[gray]unknown: the guest agent is not enabled[-]"
	default:
		return metricUnknown
	}
}

// vmDeviceHeader is the header of the devices table.
var vmDeviceHeader = []string{"KIND", "NAME", "DEVICE", "SIZE", "STATE", "DETAIL"}

// vmDeviceRows flattens the disks and the interfaces of a VM into the cells of the devices table.
func vmDeviceRows(res *vmResources, name string) [][]string {
	spec := res.specs[name]
	if spec == nil {
		return nil
	}

	var rows [][]string

	diskStatuses := res.diskStatuses(name)

	for _, disk := range spec.TypedSpec().Disks {
		rows = append(rows, vmDiskCells(disk, diskStatuses[disk.Name]))
	}

	for _, iface := range spec.TypedSpec().Interfaces {
		detail := "link " + iface.Link
		if iface.HardwareAddr != "" {
			detail += " · " + iface.HardwareAddr
		}

		rows = append(rows, []string{"nic", iface.Name, iface.Attachment, metricUnknown, metricUnknown, detail})
	}

	return rows
}

func vmDiskCells(disk hypervisor.VirtualMachineDiskSpec, status *hypervisor.VirtualMachineDiskStatus) []string {
	size, state, detail := metricUnknown, "pending", vmDiskSource(disk)

	if disk.Size > 0 {
		size = humanize.IBytes(disk.Size)
	}

	if status != nil {
		if status.TypedSpec().Size > 0 {
			size = humanize.IBytes(status.TypedSpec().Size)
		}

		state = status.TypedSpec().Phase.String()

		if status.TypedSpec().Error != "" {
			detail = strings.Join(strings.Fields(status.TypedSpec().Error), " ")
		}
	}

	return []string{"disk", disk.Name, disk.Type + "/" + disk.Bus, size, state, detail}
}

// vmDiskSource describes where the content of a disk comes from.
func vmDiskSource(disk hypervisor.VirtualMachineDiskSpec) string {
	switch {
	case disk.IsEmptyDrive():
		return "empty drive"
	case disk.Provision.FromImage != nil:
		return fmt.Sprintf("image %s/%s", disk.Provision.FromImage.Library, disk.Provision.FromImage.File)
	case disk.Provision.Blank:
		return "blank volume in pool " + disk.Pool
	default:
		return metricUnknown
	}
}
