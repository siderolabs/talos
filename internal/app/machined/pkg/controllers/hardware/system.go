// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/prometheus/procfs"
	"github.com/prometheus/procfs/sysfs"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-smbios/smbios"
	"go.uber.org/zap"

	hwadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/hardware"
	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	pkgSMBIOS "github.com/siderolabs/talos/internal/pkg/smbios"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// SystemInfoController populates CPU information of the underlying hardware.
type SystemInfoController struct {
	V1Alpha1Mode runtimetalos.Mode
	SMBIOS       *smbios.SMBIOS
}

// Name implements controller.Controller interface.
func (ctrl *SystemInfoController) Name() string {
	return "hardware.SystemInfoController"
}

// Inputs implements controller.Controller interface.
func (ctrl *SystemInfoController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MetaKeyType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MetaLoadedType,
			ID:        optional.Some(runtime.MetaLoadedID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *SystemInfoController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hardware.ProcessorType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: hardware.MemoryModuleType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: hardware.SystemInformationType,
			Kind: controller.OutputExclusive,
		},
	}
}

const memoryModuleUnknown = "UNKNOWN"

const processorUnknown = "UNKNOWN"

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *SystemInfoController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// smbios info is not available inside container, so skip the controller
	if ctrl.V1Alpha1Mode == runtimetalos.ModeContainer {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		_, err := safe.ReaderGetByID[*runtime.MetaLoaded](ctx, r, runtime.MetaLoadedID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting meta loaded resource: %w", err)
		}

		if ctrl.SMBIOS == nil {
			var s *smbios.SMBIOS

			s, err = pkgSMBIOS.GetSMBIOSInfo()
			if err != nil {
				return err
			}

			ctrl.SMBIOS = s
		}

		r.StartTrackingOutputs()

		if err := ctrl.reconcileSystemInformation(ctx, r, logger); err != nil {
			return err
		}

		if err := ctrl.reconcileProcessors(ctx, r, logger); err != nil {
			return err
		}

		if err := ctrl.reconcileMemoryModules(ctx, r, logger); err != nil {
			return err
		}

		if err := r.CleanupOutputs(
			ctx,
			resource.NewMetadata(hardware.NamespaceName, hardware.SystemInformationType, hardware.SystemInformationID, resource.VersionUndefined),
			resource.NewMetadata(hardware.NamespaceName, hardware.ProcessorType, "", resource.VersionUndefined),
			resource.NewMetadata(hardware.NamespaceName, hardware.MemoryModuleType, "", resource.VersionUndefined),
		); err != nil {
			return fmt.Errorf("failed to cleanup outputs: %w", err)
		}
	}
}

func (ctrl *SystemInfoController) reconcileSystemInformation(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	uuidRewriteRes, err := safe.ReaderGetByID[*runtime.MetaKey](ctx, r, runtime.MetaKeyTagToID(meta.UUIDOverride))
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting meta key resource: %w", err)
	}

	var uuidRewrite string

	if uuidRewriteRes != nil && uuidRewriteRes.TypedSpec().Value != "" {
		uuidRewrite = uuidRewriteRes.TypedSpec().Value

		logger.Info("using UUID rewrite", zap.String("uuid", uuidRewrite))
	}

	if err := safe.WriterModify(ctx, r, hardware.NewSystemInformation(hardware.SystemInformationID), func(res *hardware.SystemInformation) error {
		hwadapter.SystemInformation(res).Update(&ctrl.SMBIOS.SystemInformation, uuidRewrite)
		res.TypedSpec().BIOSVersion = ctrl.SMBIOS.BIOSInformation.Version

		return nil
	}); err != nil {
		return fmt.Errorf("error updating objects: %w", err)
	}

	return nil
}

func (ctrl *SystemInfoController) reconcileProcessors(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if len(ctrl.SMBIOS.ProcessorInformation) == 0 {
		return ctrl.reconcileProcessorsFromSysfs(ctx, r, logger)
	}

	var topologies []cpuTopologyInfo

	if slices.ContainsFunc(ctrl.SMBIOS.ProcessorInformation, processorNeedsTopologyFallback) {
		logger.Debug("processor information incomplete, attempting to retrieve topology from sysfs")

		var err error

		topologies, err = cpuTopologyFallback()
		if err != nil {
			return err
		}
	}

	for i, p := range ctrl.SMBIOS.ProcessorInformation {
		// replaces `CPU 0` with `CPU-0`
		id := strings.ReplaceAll(p.SocketDesignation, " ", "-")
		populated := p.Status.SocketPopulated()

		if err := safe.WriterModify(ctx, r, hardware.NewProcessorInfo(id), func(res *hardware.Processor) error {
			hwadapter.Processor(res).Update(&p)

			if populated {
				applyTopologyFallback(res.TypedSpec(), topologies, i)
			}

			return nil
		}); err != nil {
			return fmt.Errorf("error updating objects: %w", err)
		}
	}

	return nil
}

// reconcileProcessorsFromSysfs populates Processor resources purely from
// /sys/devices/system/cpu, for use when SMBIOS provides no processor
// information at all. This happens on boards with thin or incomplete
// firmware, e.g. some ARM SBCs booting through a minimal UEFI shim.
func (ctrl *SystemInfoController) reconcileProcessorsFromSysfs(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	logger.Debug("no processor information found, attempting to retrieve topology from sysfs")

	topologies, err := cpuTopologyFallback()
	if err != nil {
		return err
	}

	for i, t := range topologies {
		id := processorUnknown
		if len(topologies) > 1 {
			id = fmt.Sprintf("%s-%d", processorUnknown, i)
		}

		if err := safe.WriterModify(ctx, r, hardware.NewProcessorInfo(id), func(res *hardware.Processor) error {
			spec := res.TypedSpec()
			spec.CoreCount = t.coreCount
			spec.CoreEnabled = t.coreCount
			spec.ThreadCount = t.threadCount
			spec.MaxSpeed = t.maxSpeedMHz

			return nil
		}); err != nil {
			return fmt.Errorf("error updating objects: %w", err)
		}
	}

	return nil
}

// processorNeedsTopologyFallback reports whether a populated SMBIOS processor socket is missing
// CoreCount, ThreadCount, or MaxSpeed, and would benefit from the sysfs-derived fallback.
func processorNeedsTopologyFallback(p smbios.ProcessorInformation) bool {
	return p.Status.SocketPopulated() && (p.CoreCount == 0 || p.ThreadCount == 0 || p.MaxSpeed == 0)
}

// applyTopologyFallback fills zero CoreCount/CoreEnabled/ThreadCount/MaxSpeed fields on spec
// from the topology detected for the i-th SMBIOS processor entry, if any was detected.
func applyTopologyFallback(spec *hardware.ProcessorSpec, topologies []cpuTopologyInfo, i int) {
	if len(topologies) == 0 {
		return
	}

	t := topologies[min(i, len(topologies)-1)]

	if spec.CoreCount == 0 {
		spec.CoreCount = t.coreCount
		spec.CoreEnabled = t.coreCount
	}

	if spec.ThreadCount == 0 {
		spec.ThreadCount = t.threadCount
	}

	if spec.MaxSpeed == 0 {
		spec.MaxSpeed = t.maxSpeedMHz
	}
}

// cpuTopologyInfo describes the topology of a single physical processor
// package (socket), derived from /sys/devices/system/cpu.
type cpuTopologyInfo struct {
	coreCount   uint32
	threadCount uint32
	maxSpeedMHz uint32
}

// cpuPackageInfo accumulates the topology of a single physical processor
// package while walking /sys/devices/system/cpu.
type cpuPackageInfo struct {
	coreIDs     map[string]struct{}
	threadCount uint32
	maxSpeedMHz uint32
}

// cpuTopologyFallback derives per-socket processor topology from
// /sys/devices/system/cpu. Logical CPUs are grouped by physical_package_id,
// so multi-socket systems are still split correctly. The result is sorted by
// ascending package ID.
func cpuTopologyFallback() ([]cpuTopologyInfo, error) {
	fs, err := sysfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}

	cpus, err := fs.CPUs()
	if err != nil {
		return nil, err
	}

	packages, order, cpuPackage := groupCPUsByPackage(cpus)

	// best-effort: not all systems expose cpufreq (e.g. many VMs), so a
	// missing/unreadable cpuinfo_max_freq just leaves MaxSpeed unset, same as
	// the existing SMBIOS-derived path.
	applyMaxSpeeds(fs, packages, cpuPackage)

	sort.Slice(order, func(i, j int) bool {
		a, errA := strconv.Atoi(order[i])
		b, errB := strconv.Atoi(order[j])

		if errA == nil && errB == nil {
			return a < b
		}

		return order[i] < order[j]
	})

	result := make([]cpuTopologyInfo, 0, len(order))

	for _, id := range order {
		p := packages[id]

		result = append(result, cpuTopologyInfo{
			coreCount:   uint32(len(p.coreIDs)),
			threadCount: p.threadCount,
			maxSpeedMHz: p.maxSpeedMHz,
		})
	}

	return result, nil
}

// groupCPUsByPackage walks the topology of every CPU, grouping them by physical package ID.
// It also returns package IDs in first-seen order, and a logical-CPU-number -> package ID map.
func groupCPUsByPackage(cpus []sysfs.CPU) (map[string]*cpuPackageInfo, []string, map[string]string) {
	packages := map[string]*cpuPackageInfo{}
	cpuPackage := map[string]string{}

	var order []string

	for _, cpu := range cpus {
		topology, err := cpu.Topology()
		if err != nil {
			// topology info isn't available for this CPU (e.g. offline), skip it
			continue
		}

		cpuPackage[cpu.Number()] = topology.PhysicalPackageID

		p, ok := packages[topology.PhysicalPackageID]
		if !ok {
			p = &cpuPackageInfo{coreIDs: map[string]struct{}{}}
			packages[topology.PhysicalPackageID] = p

			order = append(order, topology.PhysicalPackageID)
		}

		p.coreIDs[topology.CoreID] = struct{}{}
		p.threadCount++
	}

	return packages, order, cpuPackage
}

// applyMaxSpeeds fills in maxSpeedMHz per package from cpufreq, where available.
func applyMaxSpeeds(fs sysfs.FS, packages map[string]*cpuPackageInfo, cpuPackage map[string]string) {
	freqs, err := fs.SystemCpufreq()
	if err != nil {
		return
	}

	for _, f := range freqs {
		if f.CpuinfoMaximumFrequency == nil {
			continue
		}

		packageID, ok := cpuPackage[f.Name]
		if !ok {
			continue
		}

		if mhz := uint32(*f.CpuinfoMaximumFrequency / 1000); mhz > packages[packageID].maxSpeedMHz {
			packages[packageID].maxSpeedMHz = mhz
		}
	}
}

func (ctrl *SystemInfoController) reconcileMemoryModules(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for _, m := range ctrl.SMBIOS.MemoryDevices {
		// the device locator alone is not always unique (e.g. some boards report
		// two `DIMM 0` modules on different banks), so suffix it with the bank
		// locator when present: `DIMM 0` + `P0 CHANNEL A` -> `DIMM-0-P0-CHANNEL-A`
		locator := m.DeviceLocator
		if m.BankLocator != "" {
			locator = m.DeviceLocator + " " + m.BankLocator
		}

		id := strings.ReplaceAll(locator, " ", "-")

		if err := safe.WriterModify(ctx, r, hardware.NewMemoryModuleInfo(id), func(res *hardware.MemoryModule) error {
			hwadapter.MemoryModule(res).Update(&m)

			return nil
		}); err != nil {
			return fmt.Errorf("error updating objects: %w", err)
		}
	}

	if len(ctrl.SMBIOS.MemoryDevices) == 0 {
		logger.Debug("no memory devices found, attempting to retrieve memory information from procfs")

		proc, err := procfs.NewDefaultFS()
		if err != nil {
			return err
		}

		info, err := proc.Meminfo()
		if err != nil {
			return err
		}

		if err := safe.WriterModify(ctx, r, hardware.NewMemoryModuleInfo(memoryModuleUnknown), func(res *hardware.MemoryModule) error {
			if info.MemTotalBytes != nil {
				hwadapter.MemoryModule(res).TypedSpec().Size = uint32(*info.MemTotal / 1024)
			}

			hwadapter.MemoryModule(res).TypedSpec().Manufacturer = memoryModuleUnknown

			return nil
		}); err != nil {
			return fmt.Errorf("error updating objects: %w", err)
		}
	}

	return nil
}
