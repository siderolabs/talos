// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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

var (
	vmRoot   = Root(constants.CgroupVirtualMachinesRoot)
	kubepods = Root(constants.CgroupKubepods)
)

// Occupant is a domain, held start claim or unreleased tasks in their actual partition.
// The caller omits an occupant only after proving its domain, claims and tasks are gone.
// Desired placement and replacement definitions are not evidence of release.
type Occupant struct {
	Name   string
	Target Target
	// Pins protects the active domain's or held start's host CPU pins until release;
	// it must not be taken from a replacement definition. Empty means unpinned.
	Pins cpuset.CPUSet
}

// Policy is one side (applied or desired) of a transition.
type Policy struct {
	// Sets holds the CPU mask of every managed target. A target absent from the desired side is
	// left as it is; a target absent from the applied side has never been bounded.
	Sets map[Target]cpuset.CPUSet
	// Exclusive names the slices which must be alone on their CPUs.
	Exclusive map[string]bool
	// KubeletReservation is the kubelet's reservedSystemCPUs; nil when the kubelet is not managed.
	KubeletReservation *cpuset.CPUSet
}

// StepKind is the kind of a plan step.
type StepKind int

// Step kinds.
const (
	// StepSetCPUs writes CPUs to the target's cpuset.cpus.
	StepSetCPUs StepKind = iota
	// StepPublishKubeletReservation stages reservedSystemCPUs. The coordinator must
	// observe kubelet configuration readback before completing the pod handoff,
	// including on retries where the reservation was already published.
	StepPublishKubeletReservation
	// StepAwaitKubepodsRelease waits until no kubepods leaf with a configured cpuset.cpus has an
	// effective set intersecting CPUs.
	StepAwaitKubepodsRelease
)

// Step is one ordered operation of a plan.
type Step struct {
	Kind   StepKind
	Target Target
	CPUs   cpuset.CPUSet
	// Reservation is the value of StepPublishKubeletReservation; nil means unmanaged.
	Reservation *cpuset.CPUSet
}

// String implements fmt.Stringer.
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

// Block explains why a transition is rejected.
type Block struct {
	Reason string
	// Consumers names the virtual machines involved, sorted.
	Consumers []string
	// CPUs are the CPUs the reason is about, if any.
	CPUs cpuset.CPUSet
}

// String implements fmt.Stringer.
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

// Result of Plan.
type Result struct {
	// Steps is the ordered plan; empty when nothing has to change or the transition is rejected.
	Steps []Step
	// Blocked is non-empty when the transition is rejected.
	Blocked []Block
}

// IsBlocked reports whether the transition is rejected.
func (r Result) IsBlocked() bool {
	return len(r.Blocked) > 0
}

// Input of Plan. The caller must recover pending writes and reject enforcement loss
// (offline applied CPUs or foreign mask drift) before planning. An absent applied
// mask inherits its parent's effective CPUs; fixed roots inherit Online.
type Input struct {
	Applied Policy
	Desired Policy
	// Occupants is the complete occupancy, including claims without a listed domain.
	Occupants []Occupant
	// Online is the set of online CPUs.
	Online cpuset.CPUSet
}

// Plan returns the ordered steps from Applied to Desired, or the reasons the transition is rejected.
//
// Steps are valid only for the supplied consumers: they do not admit new ones. The caller has to
// stop admitting starts and snapshot every running, starting and unreleased consumer before
// planning, and keep admission closed while it executes the steps.
//
// Masks shrink before they grow. Children of the virtual machine root shrink before it and grow
// after it, because their effective sets are bounded by it. Kubepods sits between the two phases:
// it widens to the union of both masks, the reservation is published, the kubelet's pinned
// containers drain from the removed CPUs, and only then is the mask capped. Each written state is
// simulated, and the plan is rejected if any of them lets a running or claimed exclusive slice
// share an effective CPU with a target that has tasks. A mask is never written empty, and a target
// with tasks is never rewritten to CPUs disjoint from its current ones.
//
// Plan neither changes a consumer's placement nor its domain: a mask change keeps every domain
// running, and a placement change remains the virtual machine runtime's to carry out.
func Plan(in Input) Result {
	occupied := occupancyOf(in.Occupants)

	if blocks := validate(in); len(blocks) > 0 {
		return Result{Blocked: blocks}
	}

	if blocks := displacedPins(in); len(blocks) > 0 {
		return Result{Blocked: blocks}
	}

	if blocks := disjointRewrites(in, occupied); len(blocks) > 0 {
		return Result{Blocked: blocks}
	}

	steps := order(in)

	if blocks := simulate(in, occupied, steps); len(blocks) > 0 {
		return Result{Blocked: blocks}
	}

	return Result{Steps: steps}
}

// validate rejects desired masks which cannot be written or leave the virtual machine root.
func validate(in Input) []Block {
	var blocks []Block

	final := finalSets(in)
	root, rootBounded := final[vmRoot]

	for _, target := range sortedTargets(in.Desired.Sets) {
		set := in.Desired.Sets[target]

		switch {
		case target.CgroupPath() == "":
			blocks = append(blocks, Block{Reason: fmt.Sprintf("%s: not a CPU partition target", target)})
		case set.IsEmpty():
			blocks = append(blocks, Block{Reason: fmt.Sprintf("%s: an empty CPU set is never written", target)})
		case !set.IsSubsetOf(in.Online):
			blocks = append(blocks, Block{
				Reason: fmt.Sprintf("%s: CPUs are offline", target),
				CPUs:   set.Difference(in.Online),
			})
		case target.Kind != KindRoot && rootBounded && !set.IsSubsetOf(root):
			blocks = append(blocks, Block{
				Reason: fmt.Sprintf("%s would leave the virtual machine root %q", target, root),
				CPUs:   set.Difference(root),
			})
		}
	}

	return blocks
}

// displacedPins protects active and held-start pins, even when replacement is desired.
func displacedPins(in Input) []Block {
	final := finalSets(in)

	var blocks []Block

	for _, consumer := range sortedOccupants(in.Occupants) {
		if consumer.Pins.IsEmpty() {
			continue
		}

		before := effectiveIn(in.Applied.Sets, consumer.Target, in.Online)
		after := effectiveIn(final, consumer.Target, in.Online)

		if lost := consumer.Pins.Intersection(before).Difference(after); !lost.IsEmpty() {
			blocks = append(blocks, Block{
				Reason:    fmt.Sprintf("host CPU pins of the domain fall outside the new allocation of %s", consumer.Target),
				Consumers: []string{consumer.Name},
				CPUs:      lost,
			})
		}
	}

	return blocks
}

// disjointRewrites rejects rewriting a fixed root, or a slice with tasks, to CPUs sharing none with
// its current ones: there is no non-empty intermediate mask to stage the move through.
func disjointRewrites(in Input, occupied occupancy) []Block {
	var blocks []Block

	for _, target := range sortedTargets(in.Desired.Sets) {
		applied, existed := in.Applied.Sets[target]
		desired := in.Desired.Sets[target]

		if !existed || !applied.Intersection(desired).IsEmpty() || target.Kind != KindRoot && len(occupied.of(target)) == 0 {
			continue
		}

		blocks = append(blocks, Block{
			Reason:    fmt.Sprintf("%s: a cgroup with tasks cannot move to disjoint CPUs; keep a CPU in common with %q", target, applied),
			Consumers: occupied.of(target),
			CPUs:      desired,
		})
	}

	return blocks
}

// order lays the steps out shrink-before-grow around the kubepods handoff.
func order(in Input) []Step {
	var shrinks, grows []Step

	for _, target := range sortedTargets(in.Desired.Sets) {
		if target == kubepods {
			continue
		}

		shrink, grow := maskSteps(target, in.Applied.Sets, in.Desired.Sets[target])

		shrinks = append(shrinks, shrink...)
		grows = append(grows, grow...)
	}

	slices.SortStableFunc(shrinks, func(a, b Step) int { return cmp.Compare(depth(b.Target), depth(a.Target)) })
	slices.SortStableFunc(grows, func(a, b Step) int { return cmp.Compare(depth(a.Target), depth(b.Target)) })

	return slices.Concat(shrinks, kubepodsSteps(in), grows)
}

func depth(target Target) int {
	if target.Kind == KindRoot {
		return 0
	}

	return 1
}

// maskSteps splits one target's change into its shrink and grow writes.
func maskSteps(target Target, applied map[Target]cpuset.CPUSet, desired cpuset.CPUSet) (shrink, grow []Step) {
	set := func(cpus cpuset.CPUSet) []Step {
		return []Step{{Kind: StepSetCPUs, Target: target, CPUs: cpus}}
	}

	current, existed := applied[target]
	kept := current.Intersection(desired)

	switch {
	case existed && current.Equals(desired):
		return nil, nil
	case !existed, kept.IsEmpty():
		// A first write or a disjoint move (of a target without tasks) has no intermediate: it
		// waits until everything else has shrunk away from its new CPUs.
		return nil, set(desired)
	case kept.Equals(desired):
		return set(desired), nil
	case kept.Equals(current):
		return nil, set(desired)
	default:
		return set(kept), set(desired)
	}
}

// kubepodsSteps stages the kubepods change: the reservation tells the kubelet where pods may go,
// the barrier proves its pinned containers left the removed CPUs, and only then is the mask capped.
func kubepodsSteps(in Input) []Step {
	desired, managed := in.Desired.Sets[kubepods]
	applied, existed := in.Applied.Sets[kubepods]

	var publish []Step

	if !cpusetPtrEqual(in.Applied.KubeletReservation, in.Desired.KubeletReservation) {
		publish = []Step{{Kind: StepPublishKubeletReservation, Reservation: in.Desired.KubeletReservation}}
	}

	if !managed || existed && desired.Equals(applied) {
		return publish
	}

	if !existed {
		// Unmanaged is not absent: existing pods may inherit every online CPU.
		// Even the first cap must wait for reservation readback and leaf release.
		applied = in.Online
	}

	var steps []Step

	if !desired.IsSubsetOf(applied) {
		steps = append(steps, Step{Kind: StepSetCPUs, Target: kubepods, CPUs: applied.Union(desired)})
	}

	steps = append(steps, publish...)

	removed := applied.Difference(desired)
	if !removed.IsEmpty() {
		steps = append(steps, Step{Kind: StepAwaitKubepodsRelease, CPUs: removed})
	}

	if !existed || !removed.IsEmpty() {
		steps = append(steps, Step{Kind: StepSetCPUs, Target: kubepods, CPUs: desired})
	}

	return steps
}

// simulate walks the steps over the applied masks and rejects the plan at the first written state
// in which an occupied exclusive slice shares an effective CPU with a target that has tasks.
func simulate(in Input, occupied occupancy, steps []Step) []Block {
	current := maps.Clone(in.Applied.Sets)
	if current == nil {
		current = map[Target]cpuset.CPUSet{}
	}

	written := false

	for _, step := range steps {
		if step.Kind != StepSetCPUs {
			continue
		}

		current[step.Target] = step.CPUs
		written = true

		if block, ok := checkExclusivity(in, current, occupied, fmt.Sprintf("after %q", step)); !ok {
			return []Block{block}
		}
	}

	// Without writes, the unchanged masks must still satisfy the desired exclusivity.
	if !written {
		if block, ok := checkExclusivity(in, current, occupied, "with the current masks"); !ok {
			return []Block{block}
		}
	}

	return nil
}

// competingTargets includes inherited targets, which have no configured mask to iterate.
func competingTargets(in Input, occupied occupancy) []Target {
	targets := finalSets(in)

	for target := range occupied {
		if _, exists := targets[target]; !exists {
			targets[target] = cpuset.New()
		}
	}

	for _, root := range config.CPUPartitionRoots() {
		targets[Root(root)] = cpuset.New()
	}

	return sortedTargets(targets)
}

func checkExclusivity(in Input, current map[Target]cpuset.CPUSet, occupied occupancy, when string) (Block, bool) {
	targets := competingTargets(in, occupied)

	for _, slice := range targets {
		if slice.Kind != KindSlice || !in.Desired.Exclusive[slice.Name] {
			continue
		}

		_, bounded := current[slice]
		if !occupied.owns(slice, bounded) {
			continue
		}

		owner := effectiveIn(current, slice, in.Online)

		for _, competitor := range targets {
			if competitor == slice || !occupied.competes(competitor) {
				continue
			}

			if overlap := owner.Intersection(effectiveIn(current, competitor, in.Online)); !overlap.IsEmpty() {
				return Block{
					Reason: fmt.Sprintf("%s, exclusive %s would share CPUs with %s; the staged ordering does not keep them apart",
						when, slice, competitor),
					Consumers: slices.Sorted(slices.Values(slices.Concat(occupied[slice], occupied[competitor]))),
					CPUs:      overlap,
				}, false
			}
		}
	}

	return Block{}, true
}

// effectiveIn resolves both inheritance and cgroup v2's disjoint-mask fallback.
// Fixed roots inherit Online; VM children inherit the VM root's effective mask.
func effectiveIn(sets map[Target]cpuset.CPUSet, target Target, online cpuset.CPUSet) cpuset.CPUSet {
	parent := online
	if target.Kind != KindRoot {
		parent = effectiveIn(sets, vmRoot, online)
	}

	if effective := sets[target].Intersection(parent); !effective.IsEmpty() {
		return effective
	}

	return parent
}

// finalSets returns the masks once the transition completes.
func finalSets(in Input) map[Target]cpuset.CPUSet {
	final := maps.Clone(in.Applied.Sets)
	if final == nil {
		final = map[Target]cpuset.CPUSet{}
	}

	maps.Copy(final, in.Desired.Sets)

	return final
}

// occupancy lists the occupying consumers by applied target.
type occupancy map[Target][]string

func occupancyOf(occupants []Occupant) occupancy {
	occupied := occupancy{}

	for _, occupant := range occupants {
		occupied[occupant.Target] = append(occupied[occupant.Target], occupant.Name)
	}

	return occupied
}

// of lists the consumers with tasks under the target; the virtual machine root covers its children.
func (o occupancy) of(target Target) []string {
	var names []string

	for t, consumers := range o {
		if t == target || target == vmRoot && t.Kind != KindRoot {
			names = append(names, consumers...)
		}
	}

	slices.Sort(names)

	return names
}

// owns protects an occupied slice, including an inherited one. A bounded allocation
// also conflicts with tasks directly in the VM root. A desired-only, unoccupied
// slice is not an allocation yet.
func (o occupancy) owns(slice Target, bounded bool) bool {
	return len(o[slice]) > 0 || bounded && len(o[vmRoot]) > 0
}

// competes reports whether the target's own tasks may share CPUs with an exclusive slice: the
// virtual machine root competes only with machines placed directly in it, not through its children.
func (o occupancy) competes(target Target) bool {
	return target.Kind == KindRoot && target != vmRoot || len(o[target]) > 0
}

func sortedTargets(sets map[Target]cpuset.CPUSet) []Target {
	return slices.SortedFunc(maps.Keys(sets), Target.less)
}

func sortedOccupants(occupants []Occupant) []Occupant {
	return slices.SortedFunc(slices.Values(occupants), func(a, b Occupant) int {
		return cmp.Compare(a.Name, b.Name)
	})
}

func cpusetPtrEqual(a, b *cpuset.CPUSet) bool {
	if a == nil || b == nil {
		return a == b
	}

	return a.Equals(*b)
}
