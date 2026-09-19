// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/rivo/tview"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// containerSort is the ordering applied to the container list.
type containerSort int

// Container list orderings, cycled through by the sort key.
const (
	containerSortName containerSort = iota
	containerSortHealth
	containerSortRestarts
	containerSortCPU
	containerSortMemory
)

// next returns the ordering the sort key switches to.
func (sort containerSort) next() containerSort {
	if sort == containerSortMemory {
		return containerSortName
	}

	return sort + 1
}

// String implements fmt.Stringer, and is what the list title displays.
func (sort containerSort) String() string {
	switch sort {
	case containerSortName:
		return "name"
	case containerSortHealth:
		return "health"
	case containerSortRestarts:
		return "restarts"
	case containerSortCPU:
		return "cpu"
	case containerSortMemory:
		return "memory"
	default:
		return "name"
	}
}

// containerRow is a single container flattened into what the list displays.
//
// It joins the COSI ContainerStatus with the metrics polled from the machine API, both of which
// may be missing independently of the other: a container that is pending has no metrics, and a
// node whose taloscontainers namespace does not exist yet reports none at all.
type containerRow struct {
	Name string
	// State is the fine-grained state, with the first unmet gate appended when pending.
	State  string
	Health containers.ContainerHealth
	// Restarts is the instance generation, i.e. restarts beyond the first start.
	Restarts uint64
	// PID is the running task's PID, zero when not running.
	PID uint32
	// CPUPercent is the share of a single core used over the last interval; NaN when unknown.
	CPUPercent float64
	// Memory is the current usage in bytes; MemoryKnown is false when unknown.
	Memory      uint64
	MemoryKnown bool
	// Image is the resolved digest once pulled, otherwise the requested reference.
	Image string
	// Error is the last failure, verbatim.
	Error string
}

// buildContainerRows joins container statuses with the polled metrics and returns the rows to
// display, filtered and ordered.
//
// cpuDiff carries the CPU time consumed over interval, which is what makes a usage percentage
// possible: Stat.CpuUsage on its own is a counter since the task started.
func buildContainerRows(
	statuses map[string]*containers.ContainerStatus,
	stats map[string]*machine.Stat,
	cpuDiff map[string]uint64,
	interval time.Duration,
	filter string,
	sortBy containerSort,
) []containerRow {
	rows := make([]containerRow, 0, len(statuses))

	for name, status := range statuses {
		spec := status.TypedSpec()

		// The metrics are keyed by the containerd container ID, which is the ID of the instance
		// currently running rather than the name of the container: RuntimeController hands the
		// instance resource ID to the runner, and the Stats API reports it back verbatim. The
		// generation is RestartCount, so the key is derivable from the status alone.
		metricID := containers.InstanceID(name, spec.RestartCount)

		row := containerRow{
			Name:       name,
			State:      containerStateText(spec),
			Health:     spec.Health,
			Restarts:   spec.RestartCount,
			PID:        spec.PID,
			CPUPercent: cpuPercent(cpuDiff, metricID, interval),
			Image:      spec.Image,
			Error:      spec.Error,
		}

		if stat := stats[metricID]; stat != nil {
			row.Memory, row.MemoryKnown = stat.MemoryUsage, true
		}

		if !matchesContainerFilter(row, filter) {
			continue
		}

		rows = append(rows, row)
	}

	sortContainerRows(rows, sortBy)

	return rows
}

// containerStateText renders the state, naming the first unmet gate when the container is waiting
// on one. "pending" alone tells an operator nothing actionable; the gate is the whole answer.
func containerStateText(spec *containers.ContainerStatusSpec) string {
	state := spec.State.String()

	if len(spec.WaitingFor) == 0 {
		return state
	}

	waiting := spec.WaitingFor[0]

	if extra := len(spec.WaitingFor) - 1; extra > 0 {
		waiting = fmt.Sprintf("%s +%d", waiting, extra)
	}

	return fmt.Sprintf("%s (%s)", state, waiting)
}

// cpuPercent converts the CPU time consumed over interval into a share of a single core,
// expressed as a percentage. It returns NaN when there is nothing to derive it from, which the
// renderer shows as a placeholder rather than a misleading zero.
func cpuPercent(cpuDiff map[string]uint64, id string, interval time.Duration) float64 {
	if interval <= 0 {
		return math.NaN()
	}

	diff, ok := cpuDiff[id]
	if !ok {
		return math.NaN()
	}

	return float64(diff) / float64(interval.Nanoseconds()) * 100.0
}

// matchesContainerFilter reports whether the row matches the list filter, which is a
// case-insensitive substring match against the name, image and state.
func matchesContainerFilter(row containerRow, filter string) bool {
	if filter == "" {
		return true
	}

	filter = strings.ToLower(filter)

	return strings.Contains(strings.ToLower(row.Name), filter) ||
		strings.Contains(strings.ToLower(row.Image), filter) ||
		strings.Contains(strings.ToLower(row.State), filter)
}

// sortContainerRows orders rows in place, always falling back to the name so that the list does
// not reshuffle between ticks when the primary key ties.
func sortContainerRows(rows []containerRow, sortBy containerSort) {
	slices.SortFunc(rows, func(a, b containerRow) int {
		var primary int

		switch sortBy {
		case containerSortName:
		case containerSortHealth:
			primary = cmp.Compare(healthSeverity(a.Health), healthSeverity(b.Health))
		case containerSortRestarts:
			primary = cmp.Compare(b.Restarts, a.Restarts)
		case containerSortCPU:
			primary = compareMetricDesc(a.CPUPercent, b.CPUPercent)
		case containerSortMemory:
			primary = compareMetricDesc(memoryMetric(a), memoryMetric(b))
		}

		if primary != 0 {
			return primary
		}

		return cmp.Compare(a.Name, b.Name)
	})
}

// memoryMetric returns the memory usage as a sortable value, NaN when unknown so that it sorts
// with the other unknowns rather than as zero.
func memoryMetric(row containerRow) float64 {
	if !row.MemoryKnown {
		return math.NaN()
	}

	return float64(row.Memory)
}

// compareMetricDesc orders two metrics from the largest down, keeping unknown (NaN) values last:
// a container the node reported no metric for is not the quietest one, it is one we know nothing
// about, and it belongs at the bottom either way.
func compareMetricDesc(a, b float64) int {
	aKnown, bKnown := !math.IsNaN(a), !math.IsNaN(b)

	switch {
	case !aKnown && !bKnown:
		return 0
	case !aKnown:
		return 1
	case !bKnown:
		return -1
	default:
		return cmp.Compare(b, a)
	}
}

// healthSeverity orders health values by how much they want an operator's attention.
func healthSeverity(health containers.ContainerHealth) int {
	switch health {
	case containers.ContainerHealthDegraded:
		return 0
	case containers.ContainerHealthPending:
		return 1
	case containers.ContainerHealthPulling:
		return 2
	case containers.ContainerHealthHealthy:
		return 3
	default:
		return 4
	}
}

// healthColor returns the tview color tag for a health value.
//
// ANSI-16 names only: the dashboard leaves the palette to the terminal so that it stays readable
// on both dark and light themes.
func healthColor(health containers.ContainerHealth) string {
	switch health {
	case containers.ContainerHealthHealthy:
		return "green"
	case containers.ContainerHealthDegraded:
		return "red"
	case containers.ContainerHealthPulling:
		return "yellow"
	case containers.ContainerHealthPending:
		return "gray"
	default:
		return "white"
	}
}

// colored wraps text in a tview color tag.
func colored(color, text string) string {
	return "[" + color + "]" + text + "[-]"
}

// formatPID renders a PID, or a placeholder when the container is not running.
func formatPID(pid uint32) string {
	if pid == 0 {
		return metricUnknown
	}

	return strconv.FormatUint(uint64(pid), 10)
}

// formatCPUPercent renders a CPU share, or a placeholder when it is unknown.
func formatCPUPercent(percent float64) string {
	if math.IsNaN(percent) {
		return metricUnknown
	}

	return fmt.Sprintf("%.1f%%", percent)
}

// formatMemory renders a memory usage, or a placeholder when it is unknown.
func formatMemory(row containerRow) string {
	if !row.MemoryKnown {
		return metricUnknown
	}

	return humanize.Bytes(row.Memory)
}

// metricUnknown is displayed in place of a metric the node did not report.
const metricUnknown = "—"

// A container name, and everything derived from a node's view of it, is machine configuration a
// node serves back: it reaches these titles as text, and tview parses its own "[tag]" markup out
// of any text it renders. Escaping here keeps a name like "[red]x[-]" printing literally instead
// of repainting the pane around it. The keyboard hints and the resource labels are ours, so they
// stay unescaped and keep their literal brackets.
//
// The log pane's label is the one exception: it is drawn by HorizontalLine, which paints its runes
// onto the screen one cell at a time and parses no markup at all, so escaping there would only
// leak the escape form into what the operator reads.

// apiDetailTitle is the detail pane's title in API mode, where a container has no COSI status.
func apiDetailTitle(name string) string {
	return fmt.Sprintf(" %s (Tab: focus, Esc: back) ", tview.Escape(name))
}

// detailTitle is the detail pane's title in Talos mode, naming the container's state and health
// when a status for it has arrived.
func detailTitle(name string, status *containers.ContainerStatus) string {
	title := tview.Escape(name)

	if status != nil {
		title = fmt.Sprintf("%s — %s / %s", title, status.TypedSpec().State, status.TypedSpec().Health)
	}

	return fmt.Sprintf(" %s (i: instances, y: YAML, Tab: focus, Esc: back) ", title)
}

// logLabel is the log pane's label, naming the container it follows and the toggles in effect.
//
// Unescaped on purpose: see the note above: HorizontalLine draws runes, not markup.
func logLabel(name string, hints []string) string {
	return fmt.Sprintf("Logs: %s (%s)", name, strings.Join(hints, ", "))
}

// yamlTitle is the YAML pane's title, naming the container and which of its resources is shown.
func yamlTitle(name, label string) string {
	return fmt.Sprintf(" %s / %s (Tab: next resource, Esc: back) ", tview.Escape(name), label)
}

// formatContainerDetail renders the field summary of one container.
//
// Any of the three inputs may be nil: the status and the spec are produced by different
// controllers, and a container that has never been started has no instance at all. Each block
// renders what it has.
func formatContainerDetail(
	status *containers.ContainerStatus,
	spec *containers.ContainerSpec,
	newest *containers.ContainerInstanceStatus,
	now time.Time,
) string {
	fields := make([][2]string, 0, containerDetailRows)

	if status != nil {
		statusSpec := status.TypedSpec()

		fields = append(fields,
			[2]string{"Image", tview.Escape(statusSpec.Image)},
			[2]string{"Runtime", fmt.Sprintf(
				"pid %s · restarts %d · exit %s",
				formatPID(statusSpec.PID),
				statusSpec.RestartCount,
				formatExitCode(statusSpec, newest),
			)},
		)

		if len(statusSpec.WaitingFor) > 0 {
			fields = append(fields, [2]string{"Waiting for", tview.Escape(strings.Join(statusSpec.WaitingFor, " · "))})
		}
	}

	fields = append(fields, [2]string{"Started", formatInstanceTimes(newest, now)})

	if spec != nil {
		containerSpec := spec.TypedSpec()

		fields = append(fields,
			[2]string{"Runs as", formatRuntimeConfig(containerSpec)},
			[2]string{"Mounts", tview.Escape(formatMounts(containerSpec.Mounts))},
			[2]string{"Depends on", tview.Escape(formatDependsOn(containerSpec.DependsOn))},
		)
	}

	if status != nil && status.TypedSpec().Error != "" {
		fields = append(fields, [2]string{"Error", colored("red", tview.Escape(status.TypedSpec().Error))})
	}

	return renderFields(fields)
}

// renderFields renders label/value pairs with the labels padded to a common width.
func renderFields(fields [][2]string) string {
	width := 0
	for _, field := range fields {
		width = max(width, len(field[0]))
	}

	var sb strings.Builder

	for _, field := range fields {
		fmt.Fprintf(&sb, "[::b]%-*s[::-]  %s\n", width, field[0], field[1])
	}

	return sb.String()
}

// formatExitCode renders how the last execution ended.
//
// The newest instance speaks for itself whenever there is one, and only once it has finished: an
// instance still running has not exited, whatever the status carries. There is no instance at all
// between generations, nor while a restart waits on a gate, and StatusController carries the exit
// code across exactly those windows - which is where an operator is most likely to be looking. A
// status that has never reported anything is left unknown rather than read back as a clean exit.
func formatExitCode(statusSpec *containers.ContainerStatusSpec, newest *containers.ContainerInstanceStatus) string {
	if newest != nil {
		if !newest.TypedSpec().Phase.Done() {
			return metricUnknown
		}

		return strconv.FormatInt(int64(newest.TypedSpec().ExitCode), 10)
	}

	if statusSpec != nil && (statusSpec.RestartCount > 0 || statusSpec.ExitCode != 0) {
		return strconv.FormatInt(int64(statusSpec.ExitCode), 10)
	}

	return metricUnknown
}

// formatInstanceTimes renders when the newest instance started and, if it has finished, how long
// it ran for.
func formatInstanceTimes(newest *containers.ContainerInstanceStatus, now time.Time) string {
	if newest == nil || newest.TypedSpec().StartedAt.IsZero() {
		return metricUnknown
	}

	spec := newest.TypedSpec()
	started := spec.StartedAt.Format(time.DateTime)

	if spec.FinishedAt.IsZero() {
		return fmt.Sprintf("%s (up %s)", started, now.Sub(spec.StartedAt).Round(time.Second))
	}

	return fmt.Sprintf("%s (ran %s, finished %s)",
		started,
		spec.FinishedAt.Sub(spec.StartedAt).Round(time.Second),
		spec.FinishedAt.Format(time.DateTime),
	)
}

// formatRuntimeConfig renders the network mode, security posture and cgroup limits on one line,
// which together are what makes a container more or less dangerous than its neighbors.
func formatRuntimeConfig(spec *containers.ContainerSpecSpec) string {
	parts := []string{"network " + networkMode(spec.Network)}

	if spec.Security.Privileged {
		parts = append(parts, colored("yellow", "privileged"))
	} else {
		parts = append(parts, "restricted")
	}

	if spec.Security.MachinedAccess {
		parts = append(parts, "machined access")
	}

	if spec.Resources.CPULimit > 0 {
		parts = append(parts, fmt.Sprintf("cpu %dm", spec.Resources.CPULimit))
	}

	if spec.Resources.MemoryLimit > 0 {
		parts = append(parts, "memory "+humanize.IBytes(spec.Resources.MemoryLimit))
	}

	return strings.Join(parts, " · ")
}

func networkMode(network containers.ContainerNetworkSpec) string {
	if network.HostNetwork {
		return "host"
	}

	return "none"
}

// formatMounts renders the resolved mounts, source first so that the destination lines up with
// what the container sees.
func formatMounts(mounts []containers.ContainerMountSpec) string {
	if len(mounts) == 0 {
		return none
	}

	rendered := make([]string, 0, len(mounts))

	for _, mount := range mounts {
		source := mount.Kind

		switch mount.Kind {
		case containers.MountKindUserVolume:
			source = "volume " + mount.VolumeID
		case containers.MountKindTmpfs:
			source = "tmpfs"

			if mount.Size > 0 {
				source += " " + humanize.IBytes(mount.Size)
			}
		case containers.MountKindHostPath:
			source = mount.Source
		}

		entry := fmt.Sprintf("%s to %s", source, mount.Destination)

		// Parenthesised rather than bracketed: square brackets are tview's color-tag syntax, and
		// escaping them here would leak the escape form into the rendered text.
		if len(mount.Options) > 0 {
			entry = fmt.Sprintf("%s (%s)", entry, strings.Join(mount.Options, ","))
		}

		rendered = append(rendered, entry)
	}

	return strings.Join(rendered, " · ")
}

// formatDependsOn renders the declared dependency gates.
func formatDependsOn(dependsOn containers.ContainerDependsOnSpec) string {
	var parts []string

	if len(dependsOn.Paths) > 0 {
		parts = append(parts, "paths "+strings.Join(dependsOn.Paths, ","))
	}

	if len(dependsOn.Networks) > 0 {
		parts = append(parts, "network "+strings.Join(dependsOn.Networks, ","))
	}

	if dependsOn.Time {
		parts = append(parts, "time")
	}

	if len(dependsOn.Containers) > 0 {
		parts = append(parts, "containers "+strings.Join(dependsOn.Containers, ","))
	}

	if len(parts) == 0 {
		return none
	}

	return strings.Join(parts, " · ")
}

// instanceRowCells renders one execution attempt as the cells of the instance history table.
func instanceRowCells(spec *containers.ContainerInstanceStatusSpec) []string {
	exitCode := metricUnknown
	if spec.Phase.Done() {
		exitCode = strconv.FormatInt(int64(spec.ExitCode), 10)
	}

	errText := spec.Error
	if errText == "" {
		errText = metricUnknown
	}

	return []string{
		strconv.FormatUint(spec.Generation, 10),
		spec.Phase.String(),
		formatPID(spec.PID),
		exitCode,
		formatTimestamp(spec.StartedAt),
		formatTimestamp(spec.FinishedAt),
		errText,
	}
}

func formatTimestamp(timestamp time.Time) string {
	if timestamp.IsZero() {
		return metricUnknown
	}

	return timestamp.Format(time.DateTime)
}

// none is displayed for an empty list of mounts or dependencies.
const none = "<none>"

// containerHealthSummary renders the counts by health shown under the list, so that a long list
// still answers "is anything wrong" at a glance.
func containerHealthSummary(rows []containerRow) string {
	if len(rows) == 0 {
		return "no containers"
	}

	counts := map[containers.ContainerHealth]int{}
	for _, row := range rows {
		counts[row.Health]++
	}

	parts := []string{fmt.Sprintf("%d containers", len(rows))}

	for _, health := range []containers.ContainerHealth{
		containers.ContainerHealthHealthy,
		containers.ContainerHealthPulling,
		containers.ContainerHealthPending,
		containers.ContainerHealthDegraded,
	} {
		if count := counts[health]; count > 0 {
			parts = append(parts, colored(healthColor(health), fmt.Sprintf("%d %s", count, health)))
		}
	}

	return strings.Join(parts, " · ")
}
