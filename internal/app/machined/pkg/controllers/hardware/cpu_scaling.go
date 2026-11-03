// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/kobject"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/trigger"
	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const cpufreqPath = "devices/system/cpu/cpufreq"

// CPUScalingController applies CPUScalingSpec resources to the cpufreq policies under sysfs and
// reports the state of each policy as a CPUScalingStatus.
//
// Applying and reporting live in one controller because the kernel raises no event when a cpufreq
// attribute changes: splitting them would leave the reported state to race the write that caused it.
type CPUScalingController struct {
	V1Alpha1Mode runtimetalos.Mode

	// SysfsPath is the sysfs mount point, defaults to /sys. Overridable for testing.
	SysfsPath string

	// ReconcileCh triggers an additional reconcile on each receive. Intended for testing only.
	ReconcileCh <-chan struct{}

	// ReadFile reads a sysfs attribute, defaults to os.ReadFile. Overridable for testing, as some
	// sysfs read errors (e.g. EBUSY from an inactive policy) cannot be reproduced with plain files.
	ReadFile func(name string) ([]byte, error)

	// defaults holds the value of each attribute before this controller first wrote it.
	defaults map[string]string
}

// Name implements controller.Controller interface.
func (ctrl *CPUScalingController) Name() string {
	return "hardware.CPUScalingController"
}

// Inputs implements controller.Controller interface.
func (ctrl *CPUScalingController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: hardware.NamespaceName,
			Type:      hardware.CPUScalingSpecType,
			Kind:      controller.InputWeak,
		},
		// `machine.sysfs`/SysfsConfig can set a governor too, via KernelParamSpecController; watching
		// its output keeps the reported state from going stale after such a write.
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.KernelParamStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *CPUScalingController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hardware.CPUScalingStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *CPUScalingController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		// cpufreq under /sys belongs to the host, and is read-only for a container
		return nil
	}

	if ctrl.SysfsPath == "" {
		ctrl.SysfsPath = "/sys"
	}

	if ctrl.ReadFile == nil {
		ctrl.ReadFile = os.ReadFile
	}

	if ctrl.defaults == nil {
		ctrl.defaults = map[string]string{}
	}

	// a hotplugged socket raises one event per logical CPU, so coalesce the burst
	rateLimitedTrigger := *trigger.NewRateLimitedTrigger(ctx, r, 1, 1)

	watcher, err := kobject.NewWatcher(logger)
	if err != nil {
		return fmt.Errorf("failed to create kobject watcher: %w", err)
	}

	watchCh := watcher.Run("cpu")
	errCh := watcher.ErrCh()

	defer watcher.Close() //nolint:errcheck

	return ctrl.runWatchLoop(ctx, r, logger, rateLimitedTrigger, watchCh, errCh)
}

func (ctrl *CPUScalingController) runWatchLoop( //nolint:gocyclo
	ctx context.Context, r controller.Runtime, logger *zap.Logger,
	rateLimitedTrigger trigger.RateLimitedTrigger, watchCh <-chan *kobject.Event, errCh <-chan error,
) error {
	rateLimitedTrigger.QueueReconcile()

	for {
		select {
		case _, ok := <-watchCh:
			if !ok {
				select {
				case err := <-errCh:
					return fmt.Errorf("error watching for CPU changes: %w", err)
				default:
					return nil
				}
			}

			rateLimitedTrigger.QueueReconcile()
		case err := <-errCh:
			return fmt.Errorf("error watching for CPU changes: %w", err)
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			if err := ctrl.reconcile(ctx, r, logger); err != nil {
				return err
			}
		case <-ctrl.ReconcileCh:
			if err := ctrl.reconcile(ctx, r, logger); err != nil {
				return err
			}
		}
	}
}

func (ctrl *CPUScalingController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if err := ctrl.applySpecs(ctx, r, logger); err != nil {
		return err
	}

	return ctrl.reportStatuses(ctx, r, logger)
}

// policies lists the cpufreq policy directories. A machine with no cpufreq driver (most VMs) has
// none, which is not an error.
func (ctrl *CPUScalingController) policies() ([]string, error) {
	return filepath.Glob(filepath.Join(ctrl.SysfsPath, cpufreqPath, "policy*"))
}

// isInactive reports whether err is the error the kernel returns for every attribute of a cpufreq
// policy whose CPUs are all offline. The policy directory stays in place until a CPU comes back,
// which raises a uevent and so a reconcile.
func isInactive(err error) bool {
	return errors.Is(err, syscall.EBUSY)
}

// cpufreqPolicy is a cpufreq policy as read from sysfs.
type cpufreqPolicy struct {
	id   string
	spec hardware.CPUScalingStatusSpec

	// microarch identifies the core design of the policy's first CPU on arm64 (the MIDR_EL1
	// implementer and part number), zero where unknown.
	microarch uint64
}

func (ctrl *CPUScalingController) reportStatuses(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	dirs, err := ctrl.policies()
	if err != nil {
		return fmt.Errorf("error listing cpufreq policies: %w", err)
	}

	policies := make([]cpufreqPolicy, 0, len(dirs))

	for _, dir := range dirs {
		id := filepath.Base(dir)

		policy, err := ctrl.readPolicy(dir)

		switch {
		case err == nil:
		case errors.Is(err, fs.ErrNotExist):
			continue
		case isInactive(err):
			// an inactive policy drives no CPU, and the kernel refuses to describe it
			continue
		default:
			// a driver presenting something unexpected should not take down every other policy
			logger.Warn("error reading cpufreq policy, skipping", zap.String("policy", id), zap.Error(err))

			continue
		}

		policy.id = id
		policies = append(policies, policy)
	}

	if err = ctrl.classifyCoreTypes(policies); err != nil {
		return fmt.Errorf("error reading CPU core types: %w", err)
	}

	r.StartTrackingOutputs()

	for _, policy := range policies {
		if err := safe.WriterModify(ctx, r, hardware.NewCPUScalingStatus(policy.id), func(res *hardware.CPUScalingStatus) error {
			*res.TypedSpec() = policy.spec

			return nil
		}); err != nil {
			return fmt.Errorf("error updating CPUScalingStatus resource %q: %w", policy.id, err)
		}
	}

	return safe.CleanupOutputs[*hardware.CPUScalingStatus](ctx, r)
}

// readPolicy reads a single cpufreq policy directory.
//
// Drivers differ in which attributes they expose, so a missing one leaves its field zero.
func (ctrl *CPUScalingController) readPolicy(dir string) (cpufreqPolicy, error) {
	var policy cpufreqPolicy

	spec := &policy.spec

	// the cpufreq core creates scaling_driver for every policy, so its absence means there is no
	// policy here (any more)
	driver, err := ctrl.readAttr(dir, "scaling_driver")
	if err != nil {
		return policy, err
	}

	if driver == "" {
		return policy, fs.ErrNotExist
	}

	spec.Driver = driver

	if err = firstError(
		readInto(dir, "affected_cpus", ctrl.readCPUFreqCPUs, &spec.AffectedCPUs),
		readInto(dir, "related_cpus", ctrl.readCPUFreqCPUs, &spec.RelatedCPUs),
		readInto(dir, "scaling_available_governors", ctrl.readFields, &spec.AvailableGovernors),
		readInto(dir, "energy_performance_available_preferences", ctrl.readFields, &spec.AvailableEPPs),
		readInto(dir, "scaling_governor", ctrl.readAttr, &spec.Governor),
		readInto(dir, "energy_performance_preference", ctrl.readAttr, &spec.EnergyPerformancePreference),
		readInto(dir, "cpuinfo_min_freq", ctrl.readUintAttr, &spec.CPUInfoMinFrequencyKhz),
		readInto(dir, "cpuinfo_max_freq", ctrl.readUintAttr, &spec.CPUInfoMaxFrequencyKhz),
		readInto(dir, "base_frequency", ctrl.readUintAttr, &spec.BaseFrequencyKhz),
		readInto(dir, "scaling_min_freq", ctrl.readUintAttr, &spec.ScalingMinFrequencyKhz),
		readInto(dir, "scaling_max_freq", ctrl.readUintAttr, &spec.ScalingMaxFrequencyKhz),
	); err != nil {
		return policy, err
	}

	// An active policy has at least one online CPU. The per-CPU attributes below exist only while
	// the CPU is online, and one going offline meanwhile leaves its field zero.
	if len(spec.AffectedCPUs) == 0 {
		return policy, nil
	}

	cpuDir := filepath.Join(ctrl.SysfsPath, "devices/system/cpu", fmt.Sprintf("cpu%d", spec.AffectedCPUs[0]))

	capacity, err := ctrl.readUintAttr(cpuDir, "cpu_capacity")
	if err != nil {
		return policy, err
	}

	spec.CPUCapacity = uint32(capacity)

	policy.microarch, err = ctrl.readMicroarch(cpuDir)

	return policy, err
}

// classifyCoreTypes sets the core type of each policy on machines whose cores are not all alike,
// leaving it empty on uniform ones.
func (ctrl *CPUScalingController) classifyCoreTypes(policies []cpufreqPolicy) error {
	coreTypes, err := ctrl.readHybridPMUs()
	if err != nil {
		return err
	}

	if len(coreTypes) > 0 {
		for i := range policies {
			if cpus := policies[i].spec.AffectedCPUs; len(cpus) > 0 {
				policies[i].spec.CoreType = coreTypes[cpus[0]]
			}
		}

		return nil
	}

	classifyByMicroarch(policies)

	return nil
}

// readHybridPMUs maps logical CPUs to a core type on Intel hybrid CPUs, which register a perf PMU
// per core type, each listing its CPUs. It is empty everywhere else.
func (ctrl *CPUScalingController) readHybridPMUs() (map[uint32]string, error) {
	coreTypes := map[uint32]string{}

	for _, pmu := range []struct {
		name     string
		coreType string
	}{
		{"cpu_core", hardware.CoreTypePerformance},
		{"cpu_atom", hardware.CoreTypeEfficiency},
		// the low power E-cores of a three-tier part, e.g. Arrow Lake-H
		{"cpu_lowpower", hardware.CoreTypeEfficiency},
	} {
		cpus, err := ctrl.readCPUList(filepath.Join(ctrl.SysfsPath, "devices", pmu.name), "cpus")
		if err != nil {
			return nil, err
		}

		for _, cpu := range cpus {
			coreTypes[cpu] = pmu.coreType
		}
	}

	return coreTypes, nil
}

// classifyByMicroarch classifies the policies of an Arm system mixing core designs (big.LITTLE,
// DynamIQ): those on the design the scheduler rates the least capable are efficiency cores, the
// rest performance cores.
//
// The scheduler capacity alone does not tell core designs apart: it also scales with the maximum
// frequency, so two clusters of the same core clocked differently have different capacities, and
// intel_pstate sets a slightly different capacity on each core of a uniform CPU.
func classifyByMicroarch(policies []cpufreqPolicy) {
	// the capacity of each core design, taken as the highest seen, as one design may be clocked
	// differently across clusters
	capacities := map[uint64]uint32{}

	for _, policy := range policies {
		if policy.microarch == 0 || policy.spec.CPUCapacity == 0 {
			// can not be ranked
			return
		}

		capacities[policy.microarch] = max(capacities[policy.microarch], policy.spec.CPUCapacity)
	}

	if len(capacities) < 2 {
		return
	}

	lowest := slices.Min(slices.Collect(maps.Values(capacities)))
	highest := slices.Max(slices.Collect(maps.Values(capacities)))

	if lowest == highest {
		// the firmware does not rate the cores (no `capacity-dmips-mhz`), so there is no telling them apart
		return
	}

	for i := range policies {
		if capacities[policies[i].microarch] == lowest {
			policies[i].spec.CoreType = hardware.CoreTypeEfficiency
		} else {
			policies[i].spec.CoreType = hardware.CoreTypePerformance
		}
	}
}

func (ctrl *CPUScalingController) applySpecs(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	specs, err := safe.ReaderListAll[*hardware.CPUScalingSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing CPUScalingSpec resources: %w", err)
	}

	desired := map[string]struct{}{}

	for spec := range specs.All() {
		id := spec.Metadata().ID()

		for _, attr := range requestedAttributes(spec.TypedSpec()) {
			desired[id+"/"+attr.name] = struct{}{}
		}

		if err = ctrl.apply(id, spec.TypedSpec(), logger); err != nil {
			return fmt.Errorf("error applying CPUScalingSpec %q: %w", id, err)
		}
	}

	// tracked per attribute, so dropping one field from a config restores only that attribute
	for key := range ctrl.defaults {
		if _, ok := desired[key]; ok {
			continue
		}

		if err = ctrl.restore(key); err != nil {
			return fmt.Errorf("error restoring cpufreq attribute %q: %w", key, err)
		}
	}

	return nil
}

// cpufreqAttribute is a sysfs attribute a spec asks to be set, with the attribute listing the
// values the driver accepts for it, if any.
type cpufreqAttribute struct {
	name      string
	value     string
	available string
}

func requestedAttributes(spec *hardware.CPUScalingSpecSpec) []cpufreqAttribute {
	return xslices.Filter([]cpufreqAttribute{
		{name: "scaling_min_freq", value: formatFreq(spec.MinFrequencyKhz)},
		{name: "scaling_max_freq", value: formatFreq(spec.MaxFrequencyKhz)},
		{name: "scaling_governor", value: spec.Governor, available: "scaling_available_governors"},
		{name: "energy_performance_preference", value: spec.EnergyPerformancePreference, available: "energy_performance_available_preferences"},
	}, func(attr cpufreqAttribute) bool { return attr.value != "" })
}

func (ctrl *CPUScalingController) apply(policy string, spec *hardware.CPUScalingSpecSpec, logger *zap.Logger) error {
	err := ctrl.applyAttributes(policy, spec, logger)
	if isInactive(err) {
		// the policy keeps its settings while inactive, and is applied again once a CPU comes back
		return nil
	}

	return err
}

func (ctrl *CPUScalingController) applyAttributes(policy string, spec *hardware.CPUScalingSpecSpec, logger *zap.Logger) error {
	dir := filepath.Join(ctrl.SysfsPath, cpufreqPath, policy)

	// the kernel refuses writes to an inactive policy as it does reads, so find out before
	// touching anything
	if err := ctrl.checkActive(dir); err != nil {
		return err
	}

	attrs := requestedAttributes(spec)

	// The kernel clamps scaling_min_freq against the current scaling_max_freq and vice versa, so a
	// window which does not overlap the old one cannot be set in one pass. Repeating the first
	// attribute converges whether the window moves up or down.
	if len(attrs) > 1 && attrs[0].name == "scaling_min_freq" && attrs[1].name == "scaling_max_freq" {
		attrs = append(attrs, attrs[0])
	}

	for _, attr := range attrs {
		if attr.available != "" {
			supported, err := ctrl.supports(dir, attr.available, attr.value)
			if err != nil {
				return err
			}

			if !supported {
				// a configuration written for different hardware should not wedge the controller
				logger.Warn("cpufreq value not supported by the driver, skipping",
					zap.String("policy", policy),
					zap.String("attribute", attr.name),
					zap.String("value", attr.value),
				)

				continue
			}
		}

		if err := ctrl.write(dir, policy, attr.name, attr.value); err != nil {
			return err
		}
	}

	return nil
}

func (ctrl *CPUScalingController) supports(dir, availableAttr, value string) (bool, error) {
	available, err := ctrl.readFields(dir, availableAttr)
	if err != nil {
		return false, err
	}

	// a driver which publishes no list accepts anything it understands
	return available == nil || slices.Contains(available, value), nil
}

// write sets a sysfs attribute, remembering its previous value the first time it is touched.
func (ctrl *CPUScalingController) write(dir, policy, name, value string) error {
	key := policy + "/" + name

	if _, ok := ctrl.defaults[key]; !ok {
		current, err := ctrl.readAttr(dir, name)
		if err != nil {
			return err
		}

		if current == "" {
			// the driver does not expose this attribute
			return nil
		}

		ctrl.defaults[key] = current
	}

	if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o644); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			delete(ctrl.defaults, key)

			return nil
		}

		return fmt.Errorf("error writing %q: %w", filepath.Join(dir, name), err)
	}

	return nil
}

func (ctrl *CPUScalingController) restore(key string) error {
	policy, name, _ := strings.Cut(key, "/")

	dir := filepath.Join(ctrl.SysfsPath, cpufreqPath, policy)
	path := filepath.Join(dir, name)

	// the kernel refuses writes to an inactive policy as it does reads, but a write which fails
	// for that reason is hard to tell from one rejecting the value
	_, err := ctrl.readAttr(dir, name)
	if err == nil {
		err = os.WriteFile(path, []byte(ctrl.defaults[key]), 0o644)
	}

	switch {
	case err == nil, errors.Is(err, fs.ErrNotExist):
	case isInactive(err):
		// keep the default to restore once a CPU of the policy comes back
		return nil
	default:
		return fmt.Errorf("error writing %q: %w", path, err)
	}

	delete(ctrl.defaults, key)

	return nil
}

// readInto reads an attribute with read and stores it in dest.
func readInto[T any](dir, name string, read func(dir, name string) (T, error), dest *T) func() error {
	return func() error {
		value, err := read(dir, name)
		if err != nil {
			return err
		}

		*dest = value

		return nil
	}
}

func firstError(fns ...func() error) error {
	for _, fn := range fns {
		if err := fn(); err != nil {
			return err
		}
	}

	return nil
}

// checkActive returns the error the kernel answers with for an inactive policy, if it is one.
func (ctrl *CPUScalingController) checkActive(dir string) error {
	_, err := ctrl.readAttr(dir, "scaling_driver")

	return err
}

// readAttr reads a sysfs attribute, returning an empty string when the driver does not expose it.
func (ctrl *CPUScalingController) readAttr(dir, name string) (string, error) {
	contents, err := ctrl.ReadFile(filepath.Join(dir, name))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}

		return "", err
	}

	return strings.TrimSpace(string(contents)), nil
}

// readFields reads a sysfs attribute holding a space-separated list, such as
// scaling_available_governors.
func (ctrl *CPUScalingController) readFields(dir, name string) ([]string, error) {
	value, err := ctrl.readAttr(dir, name)
	if err != nil || value == "" {
		return nil, err
	}

	return strings.Fields(value), nil
}

func (ctrl *CPUScalingController) readUintAttr(dir, name string) (uint64, error) {
	value, err := ctrl.readAttr(dir, name)
	if err != nil || value == "" {
		return 0, err
	}

	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing %q: %w", filepath.Join(dir, name), err)
	}

	return parsed, nil
}

// readCPUFreqCPUs reads a cpufreq CPU mask attribute (affected_cpus, related_cpus). Unlike most of
// sysfs, cpufreq prints it as space-separated CPU numbers in ascending order, e.g. `0 1 2 3`, see
// cpufreq_show_cpus() in drivers/cpufreq/cpufreq.c.
func (ctrl *CPUScalingController) readCPUFreqCPUs(dir, name string) ([]uint32, error) {
	fields, err := ctrl.readFields(dir, name)
	if err != nil || len(fields) == 0 {
		return nil, err
	}

	cpus := make([]uint32, 0, len(fields))

	for _, field := range fields {
		cpu, err := strconv.ParseUint(field, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("error parsing %q: %w", filepath.Join(dir, name), err)
		}

		cpus = append(cpus, uint32(cpu))
	}

	return cpus, nil
}

// readCPUList reads an attribute in Linux CPU list format, e.g. `0-5,34`, as printed by
// cpumap_print_to_pagebuf().
func (ctrl *CPUScalingController) readCPUList(dir, name string) ([]uint32, error) {
	value, err := ctrl.readAttr(dir, name)
	if err != nil || value == "" {
		return nil, err
	}

	set, err := cpuset.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("error parsing %q: %w", filepath.Join(dir, name), err)
	}

	return xslices.Map(set.List(), func(cpu int) uint32 { return uint32(cpu) }), nil
}

// midrCoreMask selects the implementer and part number fields of MIDR_EL1, which identify the
// core design, leaving out its variant and revision.
const midrCoreMask = 0xff00fff0

// readMicroarch reads the core design of an online CPU on arm64, and returns zero elsewhere. The
// kernel prints MIDR_EL1 as `0x%016llx`, or nothing at all if it could not read it.
func (ctrl *CPUScalingController) readMicroarch(cpuDir string) (uint64, error) {
	dir := filepath.Join(cpuDir, "regs/identification")

	value, err := ctrl.readAttr(dir, "midr_el1")
	if err != nil || value == "" {
		return 0, err
	}

	midr, err := strconv.ParseUint(value, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing %q: %w", filepath.Join(dir, "midr_el1"), err)
	}

	return midr & midrCoreMask, nil
}

func formatFreq(khz uint64) string {
	if khz == 0 {
		return ""
	}

	return strconv.FormatUint(khz, 10)
}
