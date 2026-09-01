// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/utils"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// containerNamespace is one of the containerd namespaces the screen can show.
//
// Only the first is backed by COSI resources; the other two exist purely in containerd, and are
// read through the Containers and Stats APIs the same way `talosctl containers` and
// `talosctl stats` read them.
type containerNamespace struct {
	namespace string
	driver    common.ContainerDriver
}

// containerNamespaces are cycled through by the namespace key, in the order an operator is most
// likely to want them.
var containerNamespaces = []containerNamespace{
	{constants.TalosContainersContainerdNamespace, common.ContainerDriver_CONTAINERD},
	{constants.SystemContainerdNamespace, common.ContainerDriver_CONTAINERD},
	{constants.K8sContainerdNamespace, common.ContainerDriver_CRI},
}

// apiContainerRow is a container as the Containers and Stats APIs describe it, which is all that
// is available outside the taloscontainers namespace.
type apiContainerRow struct {
	// ID is the display ID, which for a CRI container is prefixed by its pod.
	ID string
	// LogID is the ID the logs are requested with.
	LogID string
	// Nested marks a container running inside a pod sandbox, rendered indented under it.
	Nested bool

	Namespace string
	Image     string
	PID       uint32
	Status    string

	// CPUPercent is the share of a single core used since the previous poll; NaN when unknown.
	CPUPercent  float64
	Memory      uint64
	MemoryKnown bool
}

// buildAPIContainerRows joins the containers of a namespace with their stats and returns the rows
// to display, filtered and ordered by ID so that a pod's containers follow their sandbox.
func buildAPIContainerRows(
	infos []*machine.ContainerInfo,
	stats map[string]*machine.Stat,
	cpuPercent map[string]float64,
	filter string,
) []apiContainerRow {
	rows := make([]apiContainerRow, 0, len(infos))

	sorted := slices.Clone(infos)
	slices.SortFunc(sorted, func(a, b *machine.ContainerInfo) int { return cmp.Compare(a.GetId(), b.GetId()) })

	for _, info := range sorted {
		row := apiContainerRow{
			ID:         info.GetId(),
			LogID:      info.GetId(),
			Nested:     info.GetPodId() != "" && info.GetId() != info.GetPodId(),
			Namespace:  info.GetNamespace(),
			Image:      info.GetImage(),
			PID:        info.GetPid(),
			Status:     info.GetStatus(),
			CPUPercent: math.NaN(),
		}

		if percent, ok := cpuPercent[info.GetId()]; ok {
			row.CPUPercent = percent
		}

		if stat := stats[info.GetId()]; stat != nil {
			row.Memory, row.MemoryKnown = stat.MemoryUsage, true
		}

		if !matchesAPIContainerFilter(row, filter) {
			continue
		}

		rows = append(rows, row)
	}

	return rows
}

// matchesAPIContainerFilter reports whether the row matches the list filter.
func matchesAPIContainerFilter(row apiContainerRow, filter string) bool {
	if filter == "" {
		return true
	}

	filter = strings.ToLower(filter)

	return strings.Contains(strings.ToLower(row.ID), filter) ||
		strings.Contains(strings.ToLower(row.Image), filter) ||
		strings.Contains(strings.ToLower(row.Status), filter)
}

// displayID renders the ID, indenting a container under the pod sandbox it belongs to, the same
// way `talosctl containers` does.
func (row apiContainerRow) displayID() string {
	if !row.Nested {
		return row.ID
	}

	return "└─ " + row.ID
}

// formatAPIMemory renders a memory usage, or a placeholder when the node did not report one.
func formatAPIMemory(row apiContainerRow) string {
	if !row.MemoryKnown {
		return metricUnknown
	}

	return humanize.Bytes(row.Memory)
}

// apiCPUPercent derives a CPU usage percentage from two Stats samples.
//
// Stat.CpuUsage is a counter of consumed CPU time, so a percentage only exists relative to the
// previous sample; containers seen for the first time report nothing rather than a spike.
func apiCPUPercent(previous, current map[string]uint64, elapsed time.Duration) map[string]float64 {
	if elapsed <= 0 {
		return nil
	}

	percent := make(map[string]float64, len(current))

	for id, usage := range current {
		before, ok := previous[id]
		if !ok || usage < before {
			continue
		}

		percent[id] = float64(usage-before) / float64(elapsed.Nanoseconds()) * 100.0
	}

	return percent
}

// startAPIPoll begins polling the Containers and Stats APIs for the selected namespace.
//
// The dashboard's shared data source only gathers the taloscontainers namespace, since that is the
// only one every screen might want; the other namespaces are polled here, and only while they are
// actually being shown.
func (widget *ContainersGrid) startAPIPoll() {
	if widget.apiCancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(widget.ctx)
	widget.apiCancel = cancel

	widget.apiCPU = nil
	widget.apiPolledAt = time.Time{}

	// The node and the generation are captured here, on the UI goroutine: OnNodeSelect writes the
	// node, and a poll that has been canceled but not yet returned must neither read it nor write
	// anything back.
	go widget.runAPIPoll(ctx, widget.selectedNode, containerNamespaces[widget.nsIndex], widget.generation)
}

// stopAPIPoll stops a running poll, if any.
func (widget *ContainersGrid) stopAPIPoll() {
	if widget.apiCancel != nil {
		widget.apiCancel()
		widget.apiCancel = nil
	}

	widget.generation++
}

func (widget *ContainersGrid) runAPIPoll(ctx context.Context, node string, target containerNamespace, generation uint64) {
	nodeCtx := utils.NodeContext(ctx, node)

	ticker := time.NewTicker(widget.dashboard.interval)
	defer ticker.Stop()

	for {
		widget.pollAPI(nodeCtx, target, generation)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// pollAPI fetches one sample and hands it to the UI goroutine.
func (widget *ContainersGrid) pollAPI(ctx context.Context, target containerNamespace, generation uint64) {
	containersResp, err := widget.dashboard.cli.Containers(ctx, target.namespace, target.driver)
	if err != nil {
		if isCanceled(err) {
			return
		}

		widget.app.QueueUpdate(func() {
			if widget.stale(generation) {
				return
			}

			widget.showListError(formatError(err))
		})

		return
	}

	// Stats is best-effort: the list is still worth showing without the metrics.
	statsResp, _ := widget.dashboard.cli.Stats(ctx, target.namespace, target.driver) //nolint:errcheck

	polledAt := time.Now()

	var infos []*machine.ContainerInfo

	for _, message := range containersResp.GetMessages() {
		infos = append(infos, message.GetContainers()...)
	}

	stats := map[string]*machine.Stat{}
	usage := map[string]uint64{}

	for _, message := range statsResp.GetMessages() {
		for _, stat := range message.GetStats() {
			stats[stat.GetId()] = stat
			usage[stat.GetId()] = stat.GetCpuUsage()
		}
	}

	widget.app.QueueUpdate(func() {
		if widget.stale(generation) {
			return
		}

		widget.apiInfos = infos
		widget.apiStats = stats
		widget.apiPercent = apiCPUPercent(widget.apiCPU, usage, polledAt.Sub(widget.apiPolledAt))
		widget.apiCPU = usage
		widget.apiPolledAt = polledAt

		widget.refresh()
	})
}

// cycleNamespace switches the list to the next containerd namespace.
func (widget *ContainersGrid) cycleNamespace() {
	widget.stopSources()

	widget.nsIndex = (widget.nsIndex + 1) % len(containerNamespaces)

	widget.apiInfos = nil
	widget.apiStats = nil
	widget.apiPercent = nil
	widget.apiRows = nil

	widget.startSources()
	widget.renderList()
}

// namespaceLabel names the namespace currently shown, for the list title.
func (widget *ContainersGrid) namespaceLabel() string {
	return containerNamespaces[widget.nsIndex].namespace
}

// talosMode reports whether the screen is showing the containers declared via ContainerConfig,
// which are the only ones COSI knows about.
func (widget *ContainersGrid) talosMode() bool {
	return widget.nsIndex == 0
}

// renderAPIList rebuilds the list from the polled Containers and Stats data.
func (widget *ContainersGrid) renderAPIList() {
	previous := widget.selectedAPIID()

	widget.apiRows = buildAPIContainerRows(widget.apiInfos, widget.apiStats, widget.apiPercent, widget.filterText)

	widget.initTableHeader()

	for i, row := range widget.apiRows {
		for column, text := range []string{
			tview.Escape(row.Namespace),
			tview.Escape(row.displayID()),
			tview.Escape(row.Image),
			formatPID(row.PID),
			tview.Escape(row.Status),
			formatCPUPercent(row.CPUPercent),
			formatAPIMemory(row),
		} {
			widget.table.SetCell(i+1, column, &tview.TableCell{
				Text:      text,
				Align:     tview.AlignLeft,
				Color:     tcell.ColorDefault,
				Expansion: apiExpansionForColumn(column),
			})
		}
	}

	widget.summary.SetText(fmt.Sprintf("%d containers in %s", len(widget.apiRows), widget.namespaceLabel()))

	if len(widget.apiRows) == 0 {
		widget.table.SetCell(1, 0, &tview.TableCell{
			Text:          "[gray]" + widget.emptyAPIListText() + "[-]",
			NotSelectable: true,
			Expansion:     1,
		})

		return
	}

	widget.restoreAPISelection(previous)
}

// apiExpansionForColumn returns the expansion factor of an API-mode list column.
func apiExpansionForColumn(column int) int {
	switch column {
	case 1:
		return 1
	case 2:
		return 3
	default:
		return 0
	}
}

func (widget *ContainersGrid) emptyAPIListText() string {
	if widget.filterText != "" {
		return "No containers match the filter."
	}

	if widget.apiPolledAt.IsZero() {
		return "Loading..."
	}

	return fmt.Sprintf("No containers in the %s namespace.", widget.namespaceLabel())
}

// selectedAPIID returns the container the cursor is on in API mode.
func (widget *ContainersGrid) selectedAPIID() string {
	row, _ := widget.table.GetSelection()

	if row < 1 || row-1 >= len(widget.apiRows) {
		return ""
	}

	return widget.apiRows[row-1].ID
}

// restoreAPISelection selects the row of the given container, falling back to the first row.
func (widget *ContainersGrid) restoreAPISelection(id string) {
	if id != "" {
		if index := slices.IndexFunc(widget.apiRows, func(row apiContainerRow) bool { return row.ID == id }); index >= 0 {
			widget.table.Select(index+1, 0)

			return
		}
	}

	widget.table.SetOffset(0, 0)
	widget.table.Select(1, 0)
}

// refreshSelectedAPIRow re-points the detail pane at the newest sample of the container it shows.
//
// The row it renders comes from the polled list, so it is replaced wholesale on every poll; when the
// container is gone from the list, or the filter hides it, the last sample is kept rather than
// blanking the pane the operator is reading.
func (widget *ContainersGrid) refreshSelectedAPIRow() {
	index := slices.IndexFunc(widget.apiRows, func(row apiContainerRow) bool { return row.ID == widget.selected })
	if index < 0 {
		return
	}

	widget.selectedAPI = widget.apiRows[index]
}

// formatAPIContainerDetail renders the field summary of a container the API described, which is
// everything that is known about it outside COSI.
func formatAPIContainerDetail(row apiContainerRow) string {
	return renderFields([][2]string{
		{"Namespace", tview.Escape(row.Namespace)},
		{"Image", tview.Escape(row.Image)},
		{"Status", tview.Escape(row.Status)},
		{"PID", formatPID(row.PID)},
		{"CPU", formatCPUPercent(row.CPUPercent)},
		{"Memory", formatAPIMemory(row)},
	})
}
