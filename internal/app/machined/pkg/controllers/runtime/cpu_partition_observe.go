// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/cpupartition"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

var (
	vmRootTarget   = cpupartition.Root(constants.CgroupVirtualMachinesRoot)
	kubepodsTarget = cpupartition.Root(constants.CgroupKubepods)
)

// observation is everything one pass knows; it performs no writes.
type observation struct {
	spec   *runtime.CPUPartitionSpec
	status *runtime.CPUPartitionStatus
	// applied is the status as recovered from the kernel.
	applied runtime.CPUPartitionStatusSpec
	online  cpuset.CPUSet

	// actual holds the configured masks the kernel reports, managed or not.
	actual     cpupartition.Policy
	desired    cpupartition.Policy
	desiredErr error
	result     cpupartition.Result
	loss       []runtime.CPUPartitionBlock

	vms        map[string]*vmObservation
	placements map[string]*hypervisor.VirtualMachineCPUPlacement
	occupants  []cpupartition.Occupant
	// restored marks the desired masks which put a target dropped from the policy back.
	restored map[cpupartition.Target]bool
	// tasks marks the virtual machine partitions with tasks no domain or claim accounts for.
	tasks map[cpupartition.Target]bool
	// residual is set when such tasks exist; only their exit can resolve it.
	residual bool
	// unverified lists written masks whose effective set did not verify; nothing is confirmed,
	// adopted or admitted on them.
	unverified []string
}

// vmObservation is the evidence of one virtual machine, by name.
type vmObservation struct {
	spec           *hypervisor.VirtualMachineSpec
	placement      *hypervisor.VirtualMachineCPUPlacement
	domain         bool
	domainClaim    bool
	placementClaim bool
	tasks          bool
}

// occupies reports whether the machine may still have tasks in its actual partition: a domain
// listed by libvirt (including one whose removal failed or is uncertain), a held claim (a start in
// flight, or one whose outcome is unknown) or residual tasks.
func (vm *vmObservation) occupies() bool {
	return vm.domain || vm.domainClaim || vm.placementClaim || vm.tasks
}

func (obs *observation) enabled() bool {
	return obs.spec.TypedSpec().Enabled
}

func (ctrl *CPUPartitionController) observe(ctx context.Context, r controller.Runtime,
	spec *runtime.CPUPartitionSpec, status *runtime.CPUPartitionStatus,
) (*observation, error) {
	obs := &observation{spec: spec, status: status}

	var err error

	if obs.online, err = ctrl.FS.Online(); err != nil {
		return nil, fmt.Errorf("error reading online CPUs: %w", err)
	}

	if status != nil {
		obs.applied = status.TypedSpec().DeepCopy()
	}

	if err = ctrl.recoverApplied(obs); err != nil {
		return nil, err
	}

	if err = observeVirtualMachines(ctx, r, obs); err != nil {
		return nil, err
	}

	if err = ctrl.observePolicy(ctx, r, obs); err != nil {
		return nil, err
	}

	if obs.desiredErr == nil && len(obs.loss) == 0 {
		obs.result = cpupartition.Plan(cpupartition.Input{
			Applied:   obs.actual,
			Desired:   obs.desired,
			Occupants: obs.occupants,
			Online:    obs.online,
		})
	}

	return obs, nil
}

// observePolicy derives the desired policy, including restorations, the actual masks and the occupancy.
func (ctrl *CPUPartitionController) observePolicy(ctx context.Context, r controller.Runtime, obs *observation) error {
	obs.desired = cpupartition.Policy{Sets: map[cpupartition.Target]cpuset.CPUSet{}, Exclusive: map[string]bool{}}

	if obs.enabled() {
		obs.desired, obs.desiredErr = desiredPolicy(obs.spec, obs.online)
	}

	if err := obs.restoreDropped(); err != nil {
		return err
	}

	if err := ctrl.observeActual(ctx, r, obs); err != nil {
		return err
	}

	return ctrl.observeOccupants(obs)
}

// restoreDropped plans every root the policy no longer names back to its original mask, or for an
// inherited original to its parent's effective set (every online CPU), which is what the empty
// mask resolves to; release then empties it once that is possible.
func (obs *observation) restoreDropped() error {
	obs.restored = map[cpupartition.Target]bool{}

	for _, entry := range obs.applied.Targets {
		target, _ := cpupartition.ParseKey(entry.Key) //nolint:errcheck // validated during recovery

		if _, desired := obs.desired.Sets[target]; desired || target.Kind != cpupartition.KindRoot || entry.LastApplied == entry.Initial {
			continue
		}

		original, err := parseMask(entry.Initial)
		if err != nil {
			return fmt.Errorf("invalid original mask of %s: %w", entry.Key, err)
		}

		if original.IsEmpty() || !original.IsSubsetOf(obs.online) {
			original = obs.online
		}

		obs.desired.Sets[target] = original
		obs.restored[target] = true
	}

	return nil
}

// settled reports whether the desired policy is applied and no placement would change, so
// admission need not close.
func (obs *observation) settled() bool {
	if !obs.enabled() || len(obs.unverified) > 0 || len(obs.result.Steps) > 0 || !obs.applied.Phase.AdmissionOpen() || planAdmission(obs).changes() {
		return false
	}

	return !slices.ContainsFunc(obs.applied.Targets, obs.releasable)
}

// releasable reports whether a target dropped from the policy could be released now.
func (obs *observation) releasable(entry runtime.CPUPartitionTargetStatus) bool {
	target, _ := cpupartition.ParseKey(entry.Key) //nolint:errcheck // validated during recovery

	if _, desired := obs.desired.Sets[target]; desired && !obs.restored[target] {
		return false
	}

	return target.Kind == cpupartition.KindRoot || !obs.tasks[target] && !obs.placed(target)
}

// placed reports whether a placement names the target.
func (obs *observation) placed(target cpupartition.Target) bool {
	return obs.placedFor(target) != ""
}

// placedFor returns the first machine whose placement names the target.
func (obs *observation) placedFor(target cpupartition.Target) string {
	for _, name := range slices.Sorted(maps.Keys(obs.placements)) {
		if placementKey(obs.placements[name].TypedSpec()) == target.Key() {
			return name
		}
	}

	return ""
}

// waitsOnKernel reports whether only a filesystem change can resolve a rejection.
func (obs *observation) waitsOnKernel() bool {
	return len(obs.loss) > 0 || obs.residual || len(obs.unverified) > 0
}

// unattributed reports whether the target has tasks no domain, claim or placement accounts for.
func (obs *observation) unattributed(target cpupartition.Target) bool {
	return obs.tasks[target] && !obs.placed(target)
}

// recoverApplied resolves interrupted writes from the kernel and detects enforcement loss.
//
// A pending intent (Intended != LastApplied) is confirmed when the kernel holds it and forgotten
// when the kernel still holds LastApplied; any other value, like any later mismatch, is a foreign
// change which is reported and never adopted. Every applied mask naming an offline CPU is lost too.
func (ctrl *CPUPartitionController) recoverApplied(obs *observation) error {
	obs.loss = nil
	obs.applied.EnforcementLoss = nil

	for _, entry := range slices.Clone(obs.applied.Targets) {
		if err := ctrl.recoverTarget(obs, entry); err != nil {
			return err
		}
	}

	return nil
}

func (ctrl *CPUPartitionController) recoverTarget(obs *observation, entry runtime.CPUPartitionTargetStatus) error {
	target, ok := cpupartition.ParseKey(entry.Key)
	if !ok {
		return fmt.Errorf("invalid applied CPU partition target %q", entry.Key)
	}

	current, exists, err := ctrl.readMask(target)
	if err != nil {
		return err
	}

	if !exists {
		if forgetMissing(target, entry) {
			obs.applied.DeleteTarget(entry.Key)
		} else {
			obs.lose(entry.Key, "its cgroup was removed outside Talos")
		}

		return nil
	}

	if entry.Intended != entry.LastApplied {
		same, err := sameMask(current, entry.Intended)
		if err != nil {
			return fmt.Errorf("%s: %w", target, err)
		}

		if same {
			// A configured mask is not a boundary until its effective set verifies; the intent stays
			// pending, and is not drift.
			if err = ctrl.verifyEffective(target, entry.Intended); err != nil {
				obs.unverified = append(obs.unverified, err.Error())

				return nil
			}

			entry.LastApplied = entry.Intended
		}

		entry.Intended = entry.LastApplied
		obs.applied.SetTarget(entry)
	}

	return obs.checkLoss(entry, current)
}

// readMask reads the configured mask of a target's cgroup, if it exists.
func (ctrl *CPUPartitionController) readMask(target cpupartition.Target) (string, bool, error) {
	exists, err := ctrl.FS.Exists(target.CgroupPath())
	if err != nil {
		return "", false, fmt.Errorf("error checking %s: %w", target, err)
	}

	if !exists {
		return "", false, nil
	}

	current, err := ctrl.FS.CPUs(target.CgroupPath())
	if err != nil {
		return "", false, fmt.Errorf("error reading %s: %w", target, err)
	}

	return current, true, nil
}

// forgetMissing reports whether a managed cgroup which no longer exists holds no owned state: the
// kubelet recreates kubepods unbounded, and a slice which never received a mask, or whose removal
// was announced (Intended emptied), has nothing to restore.
func forgetMissing(target cpupartition.Target, entry runtime.CPUPartitionTargetStatus) bool {
	return target == kubepodsTarget || target.Kind != cpupartition.KindRoot && (entry.LastApplied == "" || entry.Intended == "")
}

func (obs *observation) checkLoss(entry runtime.CPUPartitionTargetStatus, current string) error {
	same, err := sameMask(current, entry.LastApplied)
	if err != nil {
		return fmt.Errorf("%s: %w", entry.Key, err)
	}

	if !same {
		obs.lose(entry.Key, fmt.Sprintf("its mask %q was changed outside Talos from %q", current, entry.LastApplied))

		return nil
	}

	applied, err := parseMask(entry.LastApplied)
	if err != nil {
		return fmt.Errorf("invalid applied mask of %s: %w", entry.Key, err)
	}

	if offline := applied.Difference(obs.online); !offline.IsEmpty() {
		obs.lose(entry.Key, fmt.Sprintf("applied CPUs %s are offline", offline))
	}

	return nil
}

// lose records a boundary known not to hold; keys stay sorted because status targets are.
func (obs *observation) lose(key, reason string) {
	obs.applied.EnforcementLoss = append(obs.applied.EnforcementLoss, key)
	obs.loss = append(obs.loss, runtime.CPUPartitionBlock{
		Reason: fmt.Sprintf("%s: enforcement lost, %s; new starts are refused until it is restored", key, reason),
	})
}

// observeActual reads the configured mask of every target which may bound or compete: the fixed
// roots, the shared slice and every slice the desired policy, the status or a placement names. An
// empty or missing mask inherits and is absent from the result. The kubelet reservation is the
// last one published.
func (ctrl *CPUPartitionController) observeActual(ctx context.Context, r controller.Runtime, obs *observation) error {
	obs.actual = cpupartition.Policy{Sets: map[cpupartition.Target]cpuset.CPUSet{}, Exclusive: map[string]bool{}}

	for _, name := range obs.applied.Exclusive {
		obs.actual.Exclusive[name] = true
	}

	for _, target := range ctrl.inventory(obs) {
		current, _, err := ctrl.readMask(target)
		if err != nil {
			return err
		}

		set, err := parseMask(current)
		if err != nil {
			return fmt.Errorf("invalid mask of %s: %w", target, err)
		}

		if !set.IsEmpty() {
			obs.actual.Sets[target] = set
		}
	}

	var err error

	obs.actual.KubeletReservation, err = publishedReservation(ctx, r)

	return err
}

// publishedReservation reads the last reservation published; nil means unmanaged or none yet.
func publishedReservation(ctx context.Context, r controller.Runtime) (*cpuset.CPUSet, error) {
	reservation, err := safe.ReaderGetByID[*k8s.KubeletCPUReservation](ctx, r, k8s.KubeletID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("error getting kubelet CPU reservation: %w", err)
	}

	if !reservation.TypedSpec().Managed {
		return nil, nil
	}

	reserved, err := cpuset.Parse(reservation.TypedSpec().ReservedCPUs)
	if err != nil {
		return nil, fmt.Errorf("invalid published kubelet CPU reservation %q: %w", reservation.TypedSpec().ReservedCPUs, err)
	}

	return &reserved, nil
}

func (ctrl *CPUPartitionController) inventory(obs *observation) []cpupartition.Target {
	targets := map[cpupartition.Target]struct{}{cpupartition.Shared: {}}

	for _, root := range config.CPUPartitionRoots() {
		targets[cpupartition.Root(root)] = struct{}{}
	}

	for target := range obs.desired.Sets {
		targets[target] = struct{}{}
	}

	for _, entry := range obs.applied.Targets {
		if target, ok := cpupartition.ParseKey(entry.Key); ok {
			targets[target] = struct{}{}
		}
	}

	for _, vm := range obs.vms {
		if vm.placement != nil {
			target, _ := placementTarget(vm.placement.TypedSpec()) //nolint:errcheck // validated while observing
			targets[target] = struct{}{}
		}
	}

	return slices.SortedFunc(maps.Keys(targets), compareTargets)
}

func observeVirtualMachines(ctx context.Context, r controller.Runtime, obs *observation) error {
	obs.vms = map[string]*vmObservation{}
	obs.placements = map[string]*hypervisor.VirtualMachineCPUPlacement{}

	vm := func(name string) *vmObservation {
		if obs.vms[name] == nil {
			obs.vms[name] = &vmObservation{}
		}

		return obs.vms[name]
	}

	specs, err := safe.ReaderListAll[*hypervisor.VirtualMachineSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing virtual machine specs: %w", err)
	}

	for spec := range specs.All() {
		vm(spec.Metadata().ID()).spec = spec
	}

	domains, err := safe.ReaderListAll[*hypervisor.VirtualMachineDomainStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing virtual machine domain statuses: %w", err)
	}

	for domain := range domains.All() {
		vm(domain.Metadata().ID()).domain = true
	}

	domainSpecs, err := safe.ReaderListAll[*hypervisor.VirtualMachineDomainSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing virtual machine domain specs: %w", err)
	}

	for domainSpec := range domainSpecs.All() {
		if domainSpec.Metadata().Finalizers().Has(virtualMachineRuntimeName) {
			vm(domainSpec.Metadata().ID()).domainClaim = true
		}
	}

	return observePlacements(ctx, r, obs, vm)
}

func observePlacements(ctx context.Context, r controller.Runtime, obs *observation, vm func(string) *vmObservation) error {
	placements, err := safe.ReaderListAll[*hypervisor.VirtualMachineCPUPlacement](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing CPU placements: %w", err)
	}

	for placement := range placements.All() {
		if _, err = placementTarget(placement.TypedSpec()); err != nil {
			return fmt.Errorf("CPU placement %q: %w", placement.Metadata().ID(), err)
		}

		name := placement.Metadata().ID()
		obs.placements[name] = placement
		vm(name).placement = placement
		vm(name).placementClaim = placement.Metadata().Finalizers().Has(virtualMachineRuntimeName)
	}

	return nil
}

// observeOccupants projects the actual occupancy for the planner.
//
// A machine occupies the partition of the placement its runtime holds, and the virtual machine
// root otherwise. The host CPU pins of an active domain or a held start are those of the
// definition it was started with, which no resource records: the desired specification may
// already describe its replacement. They are therefore protected as possibly any online CPU,
// so no transition takes an allowed CPU from an occupied partition. Tasks left in a slice by a
// released machine (or of unknown origin) occupy it without pins until they exit.
func (ctrl *CPUPartitionController) observeOccupants(obs *observation) error {
	obs.tasks = map[cpupartition.Target]bool{}
	occupied := map[cpupartition.Target]bool{}

	for _, name := range slices.Sorted(maps.Keys(obs.vms)) {
		vm := obs.vms[name]
		if !vm.occupies() {
			continue
		}

		target := vmRootTarget
		if vm.placementClaim {
			target, _ = placementTarget(vm.placement.TypedSpec()) //nolint:errcheck // validated while observing
		}

		occupied[target] = true
		obs.occupants = append(obs.occupants, cpupartition.Occupant{Name: name, Target: target, Pins: obs.online})
	}

	for _, target := range ctrl.inventory(obs) {
		if target.Kind == cpupartition.KindRoot || occupied[target] {
			continue
		}

		populated, err := ctrl.populated(target)
		if err != nil {
			return err
		}

		if populated {
			obs.residualTasks(target)
		}
	}

	return ctrl.unattributedTasks(obs, occupied[vmRootTarget])
}

// unattributedTasks finds tasks below the virtual machine root which no inventoried target accounts
// for: a domain scope left behind by a machine whose observation and claims are gone, or a slice no
// resource names any more. They may run on any CPU of the root, so they occupy it until they exit.
// While a machine is known to run directly in the root, unknown children are presumed to be its own.
func (ctrl *CPUPartitionController) unattributedTasks(obs *observation, rootOccupied bool) error {
	if rootOccupied {
		return nil
	}

	children, err := ctrl.FS.Children(vmRootTarget.CgroupPath())
	if err != nil {
		return fmt.Errorf("error listing %s: %w", vmRootTarget, err)
	}

	known := map[string]bool{}

	for _, target := range ctrl.inventory(obs) {
		if target.Kind != cpupartition.KindRoot {
			known[target.CgroupPath()] = true
		}
	}

	for _, child := range children {
		path := vmRootTarget.CgroupPath() + "/" + child
		if known[path] {
			continue
		}

		populated, err := ctrl.FS.Populated(path)
		if err != nil {
			return fmt.Errorf("error reading the tasks of %q: %w", path, err)
		}

		if populated {
			obs.residual = true
			obs.occupants = append(obs.occupants, cpupartition.Occupant{Name: "tasks in " + path, Target: vmRootTarget})
		}
	}

	return nil
}

// residualTasks records tasks in a virtual machine partition no domain or claim accounts for; they
// are attributed to the machine whose placement names the partition, if any.
func (obs *observation) residualTasks(target cpupartition.Target) {
	obs.residual = true
	obs.tasks[target] = true
	owner := "tasks in " + target.Key()

	for _, name := range slices.Sorted(maps.Keys(obs.vms)) {
		if vm := obs.vms[name]; vm.placement != nil && placementKey(vm.placement.TypedSpec()) == target.Key() {
			vm.tasks = true
			owner = name
		}
	}

	obs.occupants = append(obs.occupants, cpupartition.Occupant{Name: owner, Target: target})
}

// populated reports whether the target's cgroup has tasks; a missing cgroup has none.
func (ctrl *CPUPartitionController) populated(target cpupartition.Target) (bool, error) {
	exists, err := ctrl.FS.Exists(target.CgroupPath())
	if err != nil {
		return false, fmt.Errorf("error checking %s: %w", target, err)
	}

	if !exists {
		return false, nil
	}

	populated, err := ctrl.FS.Populated(target.CgroupPath())
	if err != nil {
		return false, fmt.Errorf("error reading the tasks of %s: %w", target, err)
	}

	return populated, nil
}

// desiredPolicy projects the spec into planner terms, deriving the shared remainder and the kubelet
// reservation (online minus kubepods). Any malformed entry fails the whole policy.
func desiredPolicy(spec *runtime.CPUPartitionSpec, online cpuset.CPUSet) (cpupartition.Policy, error) {
	policy := cpupartition.Policy{Sets: map[cpupartition.Target]cpuset.CPUSet{}, Exclusive: map[string]bool{}}

	for root, list := range spec.TypedSpec().Roots {
		target, ok := cpupartition.ParseKey(root)
		if !ok || target.Kind != cpupartition.KindRoot {
			return policy, fmt.Errorf("unknown root %q", root)
		}

		set, err := cpuset.Parse(list)
		if err != nil {
			return policy, fmt.Errorf("root %q: invalid CPU list %q: %w", root, list, err)
		}

		policy.Sets[target] = set
	}

	if kubepods, ok := policy.Sets[kubepodsTarget]; ok {
		policy.KubeletReservation = new(online.Difference(kubepods))
	}

	vmRoot, managed := policy.Sets[vmRootTarget]
	if !managed {
		if len(spec.TypedSpec().Slices) > 0 {
			return policy, fmt.Errorf("slices require the %q root", constants.CgroupVirtualMachinesRoot)
		}

		return policy, nil
	}

	return policy, desiredSlices(spec, policy, vmRoot)
}

func desiredSlices(spec *runtime.CPUPartitionSpec, policy cpupartition.Policy, remainder cpuset.CPUSet) error {
	for _, slice := range spec.TypedSpec().Slices {
		set, err := cpuset.Parse(slice.CPUs)
		if err != nil {
			return fmt.Errorf("slice %q: invalid CPU list %q: %w", slice.Name, slice.CPUs, err)
		}

		policy.Sets[cpupartition.Slice(slice.Name)] = set
		policy.Exclusive[slice.Name] = slice.Exclusive
		remainder = remainder.Difference(set)
	}

	if !remainder.IsEmpty() {
		policy.Sets[cpupartition.Shared] = remainder
	}

	return nil
}

func desiredManagesVirtualMachines(spec *runtime.CPUPartitionSpec) bool {
	_, ok := spec.TypedSpec().Roots[constants.CgroupVirtualMachinesRoot]

	return spec.TypedSpec().Enabled && ok
}

func exclusiveSlices(spec *runtime.CPUPartitionSpec) []string {
	var names []string

	for _, slice := range spec.TypedSpec().Slices {
		if slice.Exclusive {
			names = append(names, slice.Name)
		}
	}

	slices.Sort(names)

	return names
}

// placementTarget maps a placement back to its partition target.
func placementTarget(placement *hypervisor.VirtualMachineCPUPlacementSpec) (cpupartition.Target, error) {
	target := cpupartition.Slice(placement.Slice)

	switch {
	case placement.Partition == vmRootTarget.Partition() && placement.Slice == "":
		target = vmRootTarget
	case placement.Slice == "":
		target = cpupartition.Shared
	}

	if target.Partition() == "" || target.Partition() != placement.Partition {
		return target, fmt.Errorf("partition %q does not match slice %q", placement.Partition, placement.Slice)
	}

	return target, nil
}

func placementKey(placement *hypervisor.VirtualMachineCPUPlacementSpec) string {
	target, err := placementTarget(placement)
	if err != nil {
		return ""
	}

	return target.Key()
}

func placementRunning(placement *hypervisor.VirtualMachineCPUPlacement) bool {
	return placement.Metadata().Phase() == resource.PhaseRunning
}

// compareTargets orders roots before the shared slice before named slices.
func compareTargets(a, b cpupartition.Target) int {
	return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Name, b.Name))
}

func parseMask(list string) (cpuset.CPUSet, error) {
	if list == "" {
		return cpuset.New(), nil
	}

	return cpuset.Parse(list)
}

// sameMask compares two configured masks; the empty mask (inherit) equals only itself.
func sameMask(a, b string) (bool, error) {
	if a == "" || b == "" {
		return a == b, nil
	}

	setA, err := cpuset.Parse(a)
	if err != nil {
		return false, fmt.Errorf("invalid CPU list %q: %w", a, err)
	}

	setB, err := cpuset.Parse(b)
	if err != nil {
		return false, fmt.Errorf("invalid CPU list %q: %w", b, err)
	}

	return setA.Equals(setB), nil
}
