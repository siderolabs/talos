// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cpupartition classifies CPU partition transitions.
//
// Given the boundaries currently applied to the kernel (roots, virtual machine slices) and the
// boundaries the policy now desires, Plan returns either an ordered list of side-effect-free
// steps the coordinator can execute live, or a Blocked result naming the virtual machines and
// CPUs standing in the way. Nothing here touches the kernel, libvirt or the resource state.
//
// # What is live-safe, and what is not
//
// A cpuset boundary is enforced by the parent cgroup a domain runs in: rewriting that cgroup's
// cpuset.cpus moves its tasks without touching the domain (proven on the isolation probe: parent
// masks were changed under running guests, and roots were restricted under machined and
// virtqemud). So the mask of a partition whose path a running virtual machine keeps may shrink,
// grow live, provided every intermediate state keeps a running exclusive owner alone on its
// effective CPUs and no mask is ever written empty. A rewrite to a disjoint set of a cgroup which
// has tasks has been observed to work once (isolation probe §5) but is not proven broadly: it is
// refused until a live probe gate accepts it, so an occupied target never sees an empty
// intermediate intersection with its own old mask.
//
// Effective masks are simulated through the ancestor: a child's effective set is its mask
// intersected with the virtual machine root's, and an empty intersection falls back to the
// root's effective set (cgroup v2 behavior), which is why the root grows before its children
// and shrinks after them.
//
// A change to a virtual machine's placement (the cgroup partition it runs in, or its host CPU
// pins) is not live on Talos' supported path: both are part of the domain XML, Talos' domain
// client only defines and removes transient domains and restarts a domain whose definition
// changed, and no live retune of a running domain's partition is used (libvirt's own live
// tuning APIs are not part of this path). The user rejected such implicit restarts, so any transition needing one is
// Blocked until the operator stops the affected virtual machine: set `powerState: stopped`, wait
// until CPUPartitionStatus no longer lists the machine as blocking (the coordinator releases a
// consumer only once the runtime dropped its placement claim after removing the domain, libvirt
// no longer reports the domain, and an exclusive slice's cgroup is unpopulated; a desired
// `stopped` is not a release), apply the CPU policy change, then set `powerState: running` again.
//
// Concretely, Blocked transitions are: enabling slices while virtual machines run in the
// virtual machine root (their placement moves to shared.partition), disabling the policy or
// removing a slice while a virtual machine runs in it, moving a virtual machine between slices,
// changing a slice's CPUs under a virtual machine pinned to the old ones, and swapping the CPUs
// of two slices which both have running or not-yet-released virtual machines (no intermediate
// state keeps them apart). Everything else is planned live.
package cpupartition

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Kind of a partition target.
type Kind int

// Target kinds.
const (
	// KindRoot is one of the fixed roots (init, system, podruntime, kubepods, taloscontainers,
	// virtualMachines).
	KindRoot Kind = iota
	// KindShared is the shared.partition child of the virtual machine root.
	KindShared
	// KindSlice is a named slice child of the virtual machine root.
	KindSlice
)

// Target identifies one cgroup whose cpuset the coordinator owns.
type Target struct {
	Kind Kind
	// Name is the root name for KindRoot, the slice name for KindSlice, empty for KindShared.
	Name string
}

// Root names the target of a fixed root.
func Root(root config.CPUPartitionRoot) Target {
	return Target{Kind: KindRoot, Name: string(root)}
}

// Slice names the target of a named slice.
func Slice(name string) Target {
	return Target{Kind: KindSlice, Name: name}
}

// Shared is the target of the shared remainder.
var Shared = Target{Kind: KindShared}

func (t Target) String() string {
	switch t.Kind {
	case KindRoot:
		return "root " + t.Name
	case KindShared:
		return "shared slice"
	case KindSlice:
		return "slice " + t.Name
	default:
		return fmt.Sprintf("target(%d,%q)", t.Kind, t.Name)
	}
}

// Key is the stable string form of the target used in CPUPartitionStatus maps.
func (t Target) Key() string {
	switch t.Kind {
	case KindRoot:
		return t.Name
	case KindShared:
		return string(config.CPUPartitionRootVirtualMachines) + "/shared"
	case KindSlice:
		return string(config.CPUPartitionRootVirtualMachines) + "/" + t.Name
	default:
		return ""
	}
}

// ParseKey is the inverse of Key.
func ParseKey(key string) (Target, bool) {
	prefix := string(config.CPUPartitionRootVirtualMachines) + "/"

	if child, ok := strings.CutPrefix(key, prefix); ok {
		if child == "shared" {
			return Shared, true
		}

		return Slice(child), child != ""
	}

	for _, root := range config.CPUPartitionRoots() {
		if key == string(root) {
			return Root(root), true
		}
	}

	return Target{}, false
}

// Partition returns the libvirt resource partition path a virtual machine of this target runs in.
//
// Only KindShared and KindSlice (and the virtual machine root itself) place virtual machines.
func (t Target) Partition() string {
	switch t.Kind {
	case KindShared:
		return "/" + constants.CgroupVirtualMachines + "/" + SharedPartition
	case KindSlice:
		return "/" + constants.CgroupVirtualMachines + "/" + t.Name + ".partition"
	case KindRoot:
		return "/" + constants.CgroupVirtualMachines
	default:
		return ""
	}
}

// SharedPartition is the cgroup name of the shared slice under the virtual machine root.
const SharedPartition = "shared.partition"

func (t Target) less(other Target) int {
	return cmp.Or(cmp.Compare(t.Kind, other.Kind), cmp.Compare(t.Name, other.Name))
}

// ConsumerState is what is known about a virtual machine's occupancy of its placement.
type ConsumerState int

// Consumer states.
const (
	// ConsumerRunning: the domain runs, or a start is in flight (the domain spec is claimed).
	ConsumerRunning ConsumerState = iota
	// ConsumerUnreleased: the virtual machine is desired stopped, but its domain has not been
	// observed gone yet, or its partition still reports tasks. Treated as running for safety.
	ConsumerUnreleased
	// ConsumerReleased: no domain, no claim, its partition reports no tasks.
	ConsumerReleased
)

func (s ConsumerState) occupies() bool {
	return s != ConsumerReleased
}

// Consumer is one virtual machine as seen by the coordinator.
type Consumer struct {
	Name string
	// Applied is the placement the running domain was started in; zero for a released consumer.
	Applied Target
	// Desired is the placement the virtual machine spec now selects.
	Desired Target
	// Pins is the union of the host CPU pins in the running domain's definition; empty when unpinned.
	Pins cpuset.CPUSet
	// State of the consumer.
	State ConsumerState
	// Orphaned is set when the virtual machine spec is gone and only its placement remains.
	Orphaned bool
}

// Policy is one side (applied or desired) of a transition.
type Policy struct {
	// Sets holds the CPU mask of every managed target. A target absent from the desired side is
	// left as it is; the coordinator supplies restoration values explicitly when it wants them.
	Sets map[Target]cpuset.CPUSet
	// Exclusive names the slices which must be alone on their CPUs.
	Exclusive map[string]bool
	// KubeletReservation is the staged reservedSystemCPUs command; nil when the kubelet is not managed.
	KubeletReservation *cpuset.CPUSet
}

// StepKind is the kind of a plan step.
type StepKind int

// Step kinds, in the order the executor must handle them.
const (
	// StepSetCPUs writes CPUs to the target's cpuset.cpus and verifies cpuset.cpus.effective.
	StepSetCPUs StepKind = iota
	// StepPublishKubeletReservation stages the kubelet's reservedSystemCPUs.
	StepPublishKubeletReservation
	// StepAwaitKubepodsRelease waits until no leaf cgroup under kubepods with an explicit
	// cpuset.cpus has an effective set intersecting CPUs.
	StepAwaitKubepodsRelease
)

// Step is one ordered operation of a plan.
type Step struct {
	Kind   StepKind
	Target Target
	CPUs   cpuset.CPUSet
	// Reservation is set for StepPublishKubeletReservation; nil means unmanaged.
	Reservation *cpuset.CPUSet
}

func (s Step) String() string {
	switch s.Kind {
	case StepSetCPUs:
		return fmt.Sprintf("set %s to %q", s.Target, s.CPUs)
	case StepPublishKubeletReservation:
		if s.Reservation == nil {
			return "publish kubelet reservation unmanaged"
		}

		return fmt.Sprintf("publish kubelet reservation %q", *s.Reservation)
	case StepAwaitKubepodsRelease:
		return fmt.Sprintf("await kubepods release of %q", s.CPUs)
	default:
		return fmt.Sprintf("step(%d)", s.Kind)
	}
}

// Block explains why a transition cannot be executed live.
type Block struct {
	Reason string
	// Consumers names the virtual machines the operator has to stop, sorted.
	Consumers []string
	// CPUs are the contended CPUs, when the reason is about CPUs.
	CPUs cpuset.CPUSet
}

func (b Block) String() string {
	msg := b.Reason

	if len(b.Consumers) > 0 {
		msg += fmt.Sprintf(" (virtual machines: %s)", strings.Join(b.Consumers, ", "))
	}

	if !b.CPUs.IsEmpty() {
		msg += fmt.Sprintf(" (cpus: %s)", b.CPUs)
	}

	return msg
}

// Result of a classification.
type Result struct {
	// Steps is the ordered live plan; empty when nothing has to change.
	Steps []Step
	// Blocked is non-empty when the transition must not be executed; Steps is then empty.
	Blocked []Block
	// EnforcementLoss lists applied targets whose mask names offline CPUs. This is a safety
	// condition independent of the requested transition and is reported alongside it.
	EnforcementLoss []Target
}

// IsBlocked reports whether the transition must not be executed.
func (r Result) IsBlocked() bool {
	return len(r.Blocked) > 0
}

// Input of a classification.
type Input struct {
	Applied   Policy
	Desired   Policy
	Consumers []Consumer
	// Online is the authoritative set of online CPUs, read from sysfs.
	Online cpuset.CPUSet
}

// Plan classifies the transition from Applied to Desired.
//
// The plan is computed shrink-before-grow: children of the virtual machine root shrink before
// it, it grows before them; kubepods shrinks only after the kubelet has moved its containers off
// the removed CPUs, and grows to the union first when it both gains and loses CPUs so that the
// kubelet can place containers on the new CPUs while the old ones drain. Every step is then
// simulated against the exclusivity invariant, so a plan is returned only when no intermediate
// state lets a competitor with tasks share a CPU with a running exclusive owner.
//
//nolint:gocyclo,cyclop
func Plan(in Input) Result {
	var result Result

	// Enforcement loss is orthogonal to the requested transition.
	for _, target := range sortedTargets(in.Applied.Sets) {
		if !in.Applied.Sets[target].IsSubsetOf(in.Online) {
			result.EnforcementLoss = append(result.EnforcementLoss, target)
		}
	}

	vmRoot, vmRootBounded := in.Desired.Sets[Root(config.CPUPartitionRootVirtualMachines)]

	for _, target := range sortedTargets(in.Desired.Sets) {
		set := in.Desired.Sets[target]

		switch {
		case set.IsEmpty():
			result.Blocked = append(result.Blocked, Block{Reason: fmt.Sprintf("%s: an empty CPU set is never written", target)})
		case !set.IsSubsetOf(in.Online):
			result.Blocked = append(result.Blocked, Block{
				Reason: fmt.Sprintf("%s: CPUs are offline", target),
				CPUs:   set.Difference(in.Online),
			})
		case target.Kind != KindRoot && vmRootBounded && !set.IsSubsetOf(vmRoot):
			result.Blocked = append(result.Blocked, Block{
				Reason: fmt.Sprintf("%s would leave the virtual machine root %q", target, vmRoot),
				CPUs:   set.Difference(vmRoot),
			})
		}
	}

	if result.IsBlocked() {
		return result
	}

	// Placement changes and pins are not live: they need the operator to stop the machine.
	var placement, pinned []string

	for _, consumer := range sortedConsumers(in.Consumers) {
		if !consumer.State.occupies() {
			continue
		}

		if consumer.Applied != consumer.Desired {
			placement = append(placement, consumer.Name)

			continue
		}

		if allocation, ok := in.Desired.Sets[consumer.Desired]; ok && !consumer.Pins.IsSubsetOf(allocation) {
			pinned = append(pinned, consumer.Name)
		}
	}

	if len(placement) > 0 {
		result.Blocked = append(result.Blocked, Block{
			Reason:    "placement change requires the virtual machine to be stopped and released first",
			Consumers: placement,
		})
	}

	if len(pinned) > 0 {
		result.Blocked = append(result.Blocked, Block{
			Reason:    "host CPU pins of the running domain fall outside the new allocation; stop and release the virtual machine first",
			Consumers: pinned,
		})
	}

	if result.IsBlocked() {
		return result
	}

	if blocks := disjointRewrites(in); len(blocks) > 0 {
		result.Blocked = blocks

		return result
	}

	steps := order(in)

	if blocks := simulate(in, steps); len(blocks) > 0 {
		result.Blocked = blocks

		return result
	}

	result.Steps = steps

	return result
}

// disjointRewrites refuses to rewrite the mask of a target which has tasks to a set sharing no CPU
// with its current one: the kernel operation is not proven broadly (live probe gate), and it
// cannot be staged through a non-empty intermediate.
func disjointRewrites(in Input) []Block {
	occupied := occupiedTargets(in.Consumers)

	var blocks []Block

	for _, target := range sortedTargets(in.Desired.Sets) {
		applied, existed := in.Applied.Sets[target]
		if !existed || !applied.Intersection(in.Desired.Sets[target]).IsEmpty() {
			continue
		}

		occupants := occupantsOf(target, occupied)

		if target.Kind != KindRoot && len(occupants) == 0 {
			continue
		}

		blocks = append(blocks, Block{
			Reason:    fmt.Sprintf("%s: moving an occupied cgroup to disjoint CPUs is not supported live (live probe gate)", target),
			Consumers: occupants,
			CPUs:      in.Desired.Sets[target],
		})
	}

	return blocks
}

// occupantsOf lists the virtual machines with tasks under the target: the virtual machine root
// covers its children.
func occupantsOf(target Target, occupied map[Target][]string) []string {
	var names []string

	for t, occupants := range occupied {
		if t == target || target == Root(config.CPUPartitionRootVirtualMachines) && t.Kind != KindRoot {
			names = append(names, occupants...)
		}
	}

	slices.Sort(names)

	return names
}

func occupiedTargets(consumers []Consumer) map[Target][]string {
	occupied := map[Target][]string{}

	for _, consumer := range sortedConsumers(consumers) {
		if consumer.State.occupies() {
			occupied[consumer.Applied] = append(occupied[consumer.Applied], consumer.Name)
		}
	}

	return occupied
}

// order lays the steps out shrink-before-grow with kubepods in the middle.
//
//nolint:gocyclo
func order(in Input) []Step {
	vmRoot := Root(config.CPUPartitionRootVirtualMachines)
	kubepods := Root(config.CPUPartitionRootKubepods)

	var shrinks, grows []Step

	for _, target := range sortedTargets(in.Desired.Sets) {
		if target == kubepods {
			continue
		}

		desired := in.Desired.Sets[target]
		applied, existed := in.Applied.Sets[target]

		if existed && desired.Equals(applied) {
			continue
		}

		if !existed {
			// A new target is created empty of tasks: writing it is a grow.
			grows = append(grows, Step{Kind: StepSetCPUs, Target: target, CPUs: desired})

			continue
		}

		kept := applied.Intersection(desired)

		switch {
		case kept.IsEmpty():
			// A disjoint move has no non-empty intermediate: it is a single write in the grow phase,
			// after everything else shrank away from the new CPUs.
			grows = append(grows, Step{Kind: StepSetCPUs, Target: target, CPUs: desired})
		case kept.Equals(desired):
			shrinks = append(shrinks, Step{Kind: StepSetCPUs, Target: target, CPUs: desired})
		case kept.Equals(applied):
			grows = append(grows, Step{Kind: StepSetCPUs, Target: target, CPUs: desired})
		default:
			shrinks = append(shrinks, Step{Kind: StepSetCPUs, Target: target, CPUs: kept})
			grows = append(grows, Step{Kind: StepSetCPUs, Target: target, CPUs: desired})
		}
	}

	// Children of the virtual machine root shrink before it and grow after it.
	isChild := func(t Target) bool { return t.Kind != KindRoot }

	slices.SortStableFunc(shrinks, func(a, b Step) int {
		return cmp.Compare(boolToInt(!isChild(a.Target)), boolToInt(!isChild(b.Target)))
	})
	slices.SortStableFunc(grows, func(a, b Step) int {
		return cmp.Compare(boolToInt(a.Target != vmRoot && isChild(a.Target)), boolToInt(b.Target != vmRoot && isChild(b.Target)))
	})

	return slices.Concat(shrinks, kubepodsSteps(in), grows)
}

// kubepodsSteps stages the kubepods change: the reservation tells the kubelet where pods may go,
// the barrier proves the containers left the removed CPUs, and only then is the cap tightened.
func kubepodsSteps(in Input) []Step {
	kubepods := Root(config.CPUPartitionRootKubepods)

	var steps []Step

	desired, managed := in.Desired.Sets[kubepods]
	applied, existed := in.Applied.Sets[kubepods]

	reservationChanged := !cpusetPtrEqual(in.Applied.KubeletReservation, in.Desired.KubeletReservation)
	publish := Step{Kind: StepPublishKubeletReservation, Reservation: in.Desired.KubeletReservation}

	switch {
	case !managed:
		if reservationChanged {
			steps = append(steps, publish)
		}
	case !existed:
		// The kubelet creates its cgroup only once it runs with the staged reservation: the
		// reservation goes first, the cap waits for the cgroup.
		if reservationChanged {
			steps = append(steps, publish)
		}

		steps = append(steps, Step{Kind: StepSetCPUs, Target: kubepods, CPUs: desired})
	case desired.Equals(applied):
		if reservationChanged {
			steps = append(steps, publish)
		}
	default:
		removed := applied.Difference(desired)
		added := desired.Difference(applied)

		if !added.IsEmpty() {
			// Widen first so containers the kubelet places on the new CPUs can converge.
			steps = append(steps, Step{Kind: StepSetCPUs, Target: kubepods, CPUs: applied.Union(desired)})
		}

		if reservationChanged {
			steps = append(steps, publish)
		}

		if !removed.IsEmpty() {
			steps = append(steps,
				Step{Kind: StepAwaitKubepodsRelease, CPUs: removed},
				Step{Kind: StepSetCPUs, Target: kubepods, CPUs: desired},
			)
		}
	}

	return steps
}

// simulate walks the steps over the applied state and checks, after each write, that every
// running exclusive owner is alone on its effective CPUs against every competitor that has tasks.
// The virtual machine root is the ancestor of its children: it bounds their effective sets and is
// a competitor only when a virtual machine runs directly in it.
//
//nolint:gocyclo
func simulate(in Input, steps []Step) []Block {
	current := maps.Clone(in.Applied.Sets)

	if current == nil {
		current = map[Target]cpuset.CPUSet{}
	}

	occupied := occupiedTargets(in.Consumers)

	for _, step := range steps {
		if step.Kind != StepSetCPUs {
			continue
		}

		current[step.Target] = step.CPUs

		if block, ok := checkExclusivity(in, current, occupied, step); !ok {
			return []Block{block}
		}
	}

	return nil
}

// effective returns the effective CPU set of a target: children are bounded by the virtual
// machine root and fall back to its effective set when the intersection is empty.
func effective(current map[Target]cpuset.CPUSet, target Target) cpuset.CPUSet {
	set := current[target]

	if target.Kind == KindRoot {
		return set
	}

	root, bounded := current[Root(config.CPUPartitionRootVirtualMachines)]
	if !bounded {
		return set
	}

	if intersection := set.Intersection(root); !intersection.IsEmpty() {
		return intersection
	}

	return root
}

func checkExclusivity(in Input, current map[Target]cpuset.CPUSet, occupied map[Target][]string, step Step) (Block, bool) {
	vmRoot := Root(config.CPUPartitionRootVirtualMachines)

	for _, slice := range sortedTargets(current) {
		if slice.Kind != KindSlice || !in.Desired.Exclusive[slice.Name] {
			continue
		}

		// An unoccupied exclusive slice is not yet an owner; but a machine running directly in
		// the virtual machine root is unbounded within it and would share the slice's CPUs with
		// whoever is admitted next, so the slice cannot be carved while such a machine runs.
		if len(occupied[slice]) == 0 && len(occupied[vmRoot]) == 0 {
			continue
		}

		if competitor, overlap, ok := findCompetitor(current, occupied, slice); ok {
			return Block{
				Reason: fmt.Sprintf("after %q, exclusive %s would share CPUs with %s; no live ordering keeps them apart",
					step, slice, competitor),
				Consumers: slices.Sorted(slices.Values(slices.Concat(occupied[slice], occupied[competitor]))),
				CPUs:      overlap,
			}, false
		}
	}

	return Block{}, true
}

// hasTasks reports whether a target holds tasks: fixed roots always do, the virtual machine root
// only for machines placed directly in it, a slice while a consumer occupies it.
func hasTasks(occupied map[Target][]string, t Target) bool {
	return t.Kind == KindRoot && t != Root(config.CPUPartitionRootVirtualMachines) || len(occupied[t]) > 0
}

// findCompetitor returns the first target with tasks whose effective set overlaps the slice's.
func findCompetitor(current map[Target]cpuset.CPUSet, occupied map[Target][]string, slice Target) (Target, cpuset.CPUSet, bool) {
	owner := effective(current, slice)

	for _, competitor := range sortedTargets(current) {
		if competitor == slice || !hasTasks(occupied, competitor) {
			continue
		}

		if overlap := owner.Intersection(effective(current, competitor)); !overlap.IsEmpty() {
			return competitor, overlap, true
		}
	}

	return Target{}, cpuset.New(), false
}

func sortedTargets(sets map[Target]cpuset.CPUSet) []Target {
	return slices.SortedFunc(maps.Keys(sets), Target.less)
}

func sortedConsumers(consumers []Consumer) []Consumer {
	return slices.SortedFunc(slices.Values(consumers), func(a, b Consumer) int {
		return cmp.Compare(a.Name, b.Name)
	})
}

func cpusetPtrEqual(a, b *cpuset.CPUSet) bool {
	if a == nil || b == nil {
		return a == b
	}

	return a.Equals(*b)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}

	return 0
}
