// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/components"
	"github.com/siderolabs/talos/internal/pkg/dashboard/containerlogdata"
	"github.com/siderolabs/talos/internal/pkg/dashboard/utils"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/redactable"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

const (
	pageVMList   = "vm-list"
	pageVMDetail = "vm-detail"
	pageVMYAML   = "vm-yaml"

	// vmWatchKinds is how many resource kinds are watched, i.e. how many bootstrap events to wait for.
	vmWatchKinds = 6

	// vmLogNamespace is the namespace the machine API serves VM logs from.
	vmLogNamespace = "system"
)

// VMsGrid implements the VMs screen: a VM list, a detail pane with the serial console log, and raw
// YAML, with power actions on the selected VM. It mirrors ContainersGrid.
type VMsGrid struct {
	tview.Grid

	app       *tview.Application
	dashboard *Dashboard
	ctx       context.Context //nolint:containedctx // used to stop/start watches

	pages *tview.Pages
	level int // 0=list, 1=detail, 2=yaml

	// Level 0: VM list
	*listPane
	sortBy vmSort

	// Watched resources.
	res          *vmResources
	watchCancel  context.CancelFunc
	bootstrapped int

	// rows are the VMs currently displayed, in display order (row-1 indexed).
	rows []vmRow

	// Level 1: detail and logs
	detailWrapper *tview.Grid
	detailView    *tview.TextView
	deviceTable   *tview.Table
	showDevices   bool
	logViewer     *components.LogViewer
	logSource     *containerlogdata.Source
	logSession    uint64
	logStreaming  bool
	selected      string

	// Level 2: YAML. yamlName is the VM shown, and yamlReturn the level Esc goes back to: the YAML
	// can be opened from the list as well as from the detail.
	yamlView   *tview.TextView
	yamlKind   int
	yamlName   string
	yamlReturn int

	// message is the outcome of the last action, shown for a while; messageSeq tells which message
	// the timer that clears it belongs to.
	message    string
	messageSeq uint64
	confirming bool

	selectedNode string
	active       bool
}

// NewVMsGrid initializes VMsGrid.
func NewVMsGrid(ctx context.Context, dashboard *Dashboard) *VMsGrid {
	widget := &VMsGrid{
		Grid:      *tview.NewGrid(),
		app:       dashboard.app,
		dashboard: dashboard,
		ctx:       ctx,
		res:       newVMResources(),
		logSource: containerlogdata.NewSource(dashboard.cli),
	}

	widget.SetRows(0).SetColumns(0)

	widget.initList()
	widget.initDetail()
	widget.initYAML()

	widget.pages = tview.NewPages()
	widget.pages.AddPage(pageVMList, widget.listWrapper, true, true)
	widget.pages.AddPage(pageVMDetail, widget.detailWrapper, true, false)
	widget.pages.AddPage(pageVMYAML, widget.yamlView, true, false)

	widget.AddItem(widget.pages, 0, 0, 1, 1, 0, 0, true)

	widget.SetInputCapture(widget.handleKey)

	widget.renderList()

	go consumeLogs(ctx, widget.app, widget.logSource.LogCh, widget.writeLog)

	return widget
}

func (widget *VMsGrid) initList() {
	widget.listPane = newListPane(widget.openDetail, widget.renderList, widget.deactivateFilter)
}

func (widget *VMsGrid) initDetail() {
	widget.detailView = tview.NewTextView()
	widget.detailView.SetDynamicColors(true)
	widget.detailView.SetBorder(true)

	widget.deviceTable = newSelectableTable()
	widget.deviceTable.SetTitle(" Devices (i: back to details) ")

	widget.logViewer = components.NewLogViewer(widget.app)

	widget.detailWrapper = newDetailGrid(widget.detailView, vmDetailRows, widget.logViewer)
}

func (widget *VMsGrid) initYAML() {
	widget.yamlView = tview.NewTextView()
	widget.yamlView.SetBorder(true)
	widget.yamlView.SetScrollable(true)
	widget.yamlView.SetDynamicColors(true)
}

// onScreenSelect implements the screenSelectListener interface.
func (widget *VMsGrid) onScreenSelect(active bool) {
	widget.active = active

	if !active {
		// Nothing is visible, so stop the watch and the logs; they restart on the next activation.
		widget.stopWatch()
		widget.stopLogs()
		widget.deactivateFilter(false)
		widget.dismissConfirm()

		return
	}

	widget.startWatch()

	if widget.level == 1 {
		widget.startLogs()
	}

	widget.focusLevel()
}

// OnNodeSelect implements the NodeSelectListener interface.
func (widget *VMsGrid) OnNodeSelect(node string) {
	if node == widget.selectedNode {
		return
	}

	widget.selectedNode = node

	// Drop back to the list rather than showing another node's VM under the same name.
	widget.stopWatch()
	widget.stopLogs()
	widget.deactivateFilter(false)
	widget.dismissConfirm()

	widget.message = ""
	widget.res = newVMResources()
	widget.selected = ""
	widget.level = 0
	widget.showDevices = false

	widget.pages.SwitchToPage(pageVMList)
	widget.logViewer.Reset()
	widget.renderList()

	if widget.active {
		widget.startWatch()
		widget.focusLevel()
	}
}

// focusLevel gives the focus to the primitive of the current level.
func (widget *VMsGrid) focusLevel() {
	switch widget.level {
	case 0:
		widget.app.SetFocus(widget.table)
	case 1:
		widget.app.SetFocus(widget.logViewer)
	case 2:
		widget.app.SetFocus(widget.yamlView)
	}
}

// handleKey handles the screen-level shortcuts, passing everything through while a text input has
// the focus.
//
//nolint:gocyclo,cyclop
func (widget *VMsGrid) handleKey(event *tcell.EventKey) *tcell.EventKey {
	if _, isInput := widget.app.GetFocus().(*tview.InputField); isInput || widget.confirming {
		return event
	}

	if event.Key() == tcell.KeyEscape {
		widget.goBack()

		return nil
	}

	if action, ok := vmActionForKey(event.Rune()); ok && widget.level <= 1 {
		widget.runAction(action)

		return nil
	}

	switch event.Rune() {
	case '/':
		if widget.level == 0 {
			widget.activateFilter()

			return nil
		}
	case 's':
		if widget.level == 0 {
			widget.sortBy = widget.sortBy.next()
			widget.renderList()

			return nil
		}
	case 'y':
		if widget.level <= 1 {
			widget.openYAML()

			return nil
		}
	case 'i':
		if widget.level == 1 {
			widget.toggleDevices()

			return nil
		}
	case ' ':
		if widget.level == 1 {
			widget.logViewer.SetFollow(!widget.logViewer.Follow())
			widget.renderLogLabel()

			return nil
		}
	case 'g':
		widget.scrollToStart()

		return nil
	case 'G':
		widget.scrollToEnd()

		return nil
	}

	if event.Key() == tcell.KeyTab && widget.level == 1 {
		widget.toggleDetailFocus()

		return nil
	}

	if event.Key() == tcell.KeyTab && widget.level == 2 {
		widget.showYAML(widget.yamlKind + 1)

		return nil
	}

	return event
}

// toggleDevices swaps the top detail pane between the fields and the devices, carrying the focus over.
func (widget *VMsGrid) toggleDevices() {
	focused := widget.app.GetFocus()
	topWasFocused := focused == tview.Primitive(widget.detailView) || focused == tview.Primitive(widget.deviceTable)

	widget.showDevices = !widget.showDevices

	widget.renderDetail()

	if topWasFocused {
		widget.app.SetFocus(widget.topDetailPane())
	}
}

// topDetailPane returns the primitive currently occupying the top row of the detail level.
func (widget *VMsGrid) topDetailPane() tview.Primitive { //nolint:ireturn
	if widget.showDevices {
		return widget.deviceTable
	}

	return widget.detailView
}

// scrollToStart jumps to the top of whatever the current level shows.
func (widget *VMsGrid) scrollToStart() {
	switch widget.level {
	case 0:
		if widget.table.GetRowCount() > 1 {
			widget.table.SetOffset(0, 0)
			widget.table.Select(1, 0)
		}
	case 1:
		widget.logViewer.ScrollToBeginning()
		widget.renderLogLabel()
	case 2:
		widget.yamlView.ScrollToBeginning()
	}
}

// scrollToEnd jumps to the bottom of whatever the current level shows.
func (widget *VMsGrid) scrollToEnd() {
	switch widget.level {
	case 0:
		if rowCount := widget.table.GetRowCount(); rowCount > 1 {
			widget.table.Select(rowCount-1, 0)
		}
	case 1:
		widget.logViewer.ScrollToEnd()
		widget.renderLogLabel()
	case 2:
		widget.yamlView.ScrollToEnd()
	}
}

// toggleDetailFocus moves the focus between the top pane and the logs.
func (widget *VMsGrid) toggleDetailFocus() {
	top := widget.topDetailPane()

	if widget.app.GetFocus() == top {
		widget.app.SetFocus(widget.logViewer)

		return
	}

	widget.app.SetFocus(top)
}

// activateFilter shows the filter input below the list.
func (widget *VMsGrid) activateFilter() {
	widget.showFilter(widget.app)
}

// deactivateFilter hides the filter input. If clearText is true, the filter is also cleared.
func (widget *VMsGrid) deactivateFilter(clearText bool) {
	wasActive, rerender := widget.hideFilter(clearText)

	if rerender {
		widget.renderList()
	}

	if wasActive && widget.active && widget.level == 0 {
		widget.app.SetFocus(widget.table)
	}
}

// goBack returns to the previous navigation level.
func (widget *VMsGrid) goBack() {
	switch widget.level {
	case 2:
		widget.leaveYAML()
	case 1:
		widget.leaveDetail()
	default:
		if widget.filterText != "" {
			widget.deactivateFilter(true)
		}
	}

	widget.focusLevel()
}

// leaveYAML returns from the YAML to the level it was opened from.
func (widget *VMsGrid) leaveYAML() {
	widget.level = widget.yamlReturn

	if widget.level == 1 {
		widget.pages.SwitchToPage(pageVMDetail)

		return
	}

	// The list is not rendered while it is hidden, so catch up on what changed meanwhile.
	widget.pages.SwitchToPage(pageVMList)
	widget.renderList()
}

// leaveDetail returns from the detail to the list, and stops following the logs.
func (widget *VMsGrid) leaveDetail() {
	widget.stopLogs()
	widget.logViewer.Reset()

	widget.level = 0
	widget.selected = ""
	widget.showDevices = false

	widget.pages.SwitchToPage(pageVMList)
}

// initTableHeader sets the header row of the VM list.
func (widget *VMsGrid) initTableHeader() {
	widget.table.Clear()

	for i, name := range []string{"NAME", "POWER", "STAGE", "VCPUS", "MEM", "ADDRESSES", "ERROR"} {
		widget.table.SetCell(0, i, headerCell(name))
	}
}

// renderList rebuilds the VM list from the watched resources.
func (widget *VMsGrid) renderList() {
	widget.table.SetTitle(fmt.Sprintf(
		" VMs — by %s (Enter: details, /: filter, s: sort, y: YAML, S/T/K/R: start/stop/kill/reboot) ", widget.sortBy,
	))

	// Remember the selected VM rather than the row: sorting and updates move rows.
	previous := widget.selectedName()

	widget.rows = buildVMRows(widget.res, widget.filterText, widget.sortBy)

	widget.initTableHeader()

	for i, row := range widget.rows {
		color := row.Health.color()

		for column, text := range []string{
			tview.Escape(row.Name),
			colored(color, tview.Escape(row.Power)),
			tview.Escape(row.Stage.String()),
			formatVMVCPUs(row),
			formatVMMemory(row),
			tview.Escape(formatVMAddresses(row.Addresses)),
			colored("red", tview.Escape(formatVMError(row.Error))),
		} {
			widget.table.SetCell(i+1, column, &tview.TableCell{
				Text:      text,
				Align:     tview.AlignLeft,
				Color:     tcell.ColorDefault,
				Expansion: vmColumnExpansion(column),
			})
		}
	}

	widget.summary.SetText(joinMessage(vmSummary(widget.rows, widget.res.shuttingDown()), widget.message))

	if len(widget.rows) == 0 {
		widget.table.SetCell(1, 0, &tview.TableCell{
			Text:          "[gray]" + widget.emptyListText() + "[-]",
			NotSelectable: true,
			Expansion:     1,
		})

		return
	}

	widget.restoreSelection(previous)
}

// vmColumnExpansion makes the name and error columns grow.
func vmColumnExpansion(column int) int {
	switch column {
	case 0:
		return 1
	case 6:
		return 3
	default:
		return 0
	}
}

// emptyListText explains an empty list; without a VirtualMachineConfig it is the normal state.
func (widget *VMsGrid) emptyListText() string {
	switch {
	case widget.filterText != "":
		return "No VMs match the filter."
	case widget.bootstrapped < vmWatchKinds:
		return "Loading..."
	default:
		return "No VMs declared. Add a VirtualMachineConfig document to the machine configuration."
	}
}

// restoreSelection selects the row of the named VM, falling back to the first row when it is gone.
func (widget *VMsGrid) restoreSelection(name string) {
	if name != "" {
		if index := slices.IndexFunc(widget.rows, func(row vmRow) bool { return row.Name == name }); index >= 0 {
			widget.table.Select(index+1, 0)

			return
		}
	}

	widget.table.SetOffset(0, 0)
	widget.table.Select(1, 0)
}

// selectedName returns the VM the cursor is on, or an empty string when the list is empty.
func (widget *VMsGrid) selectedName() string {
	row, _ := widget.table.GetSelection()

	if row < 1 || row-1 >= len(widget.rows) {
		return ""
	}

	return widget.rows[row-1].Name
}

// showListError replaces the list with an error message.
func (widget *VMsGrid) showListError(msg string) {
	widget.initTableHeader()
	widget.table.SetCell(1, 0, &tview.TableCell{
		Text:          fmt.Sprintf("[red]%s[-]", tview.Escape(msg)),
		NotSelectable: true,
		Expansion:     1,
	})
	widget.summary.SetText("")
}

// startWatch begins watching the hypervisor resources of the selected node.
func (widget *VMsGrid) startWatch() {
	if widget.watchCancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(widget.ctx)
	widget.watchCancel = cancel

	// Start from nothing: the bootstrap does not replay what was destroyed while no watch ran.
	widget.res = newVMResources()

	go widget.runWatch(ctx)
}

// stopWatch cancels a running watch, if any.
func (widget *VMsGrid) stopWatch() {
	if widget.watchCancel != nil {
		widget.watchCancel()
		widget.watchCancel = nil
	}

	widget.bootstrapped = 0
}

// runWatch watches all the hypervisor kinds at once, accepting redaction: VirtualMachineSpec is
// sensitive, and without it non-admins get PermissionDenied.
func (widget *VMsGrid) runWatch(ctx context.Context) {
	nodeCtx := redactable.WithMode(utils.NodeContext(ctx, widget.selectedNode), redactable.ModeAccept)

	watchKinds(
		nodeCtx,
		widget.dashboard.cli.State(),
		hypervisor.NamespaceName,
		[]resource.Type{
			hypervisor.VirtualMachineSpecType,
			hypervisor.VirtualMachineStatusType,
			hypervisor.VirtualMachineDomainStatusType,
			hypervisor.VirtualMachineDiskStatusType,
			hypervisor.VirtualMachinePowerOverrideType,
			hypervisor.VirtualMachineLifecycleType,
		},
		func(event state.Event) { widget.handleWatchEvent(ctx, event) },
		func(err error) {
			widget.app.QueueUpdate(func() {
				if ctx.Err() != nil {
					return
				}

				widget.showListError(formatError(err))
			})
		},
	)
}

// handleWatchEvent applies a watch event on the UI goroutine, dropping events queued before ctx was
// canceled.
func (widget *VMsGrid) handleWatchEvent(ctx context.Context, event state.Event) {
	widget.app.QueueUpdate(func() {
		if ctx.Err() != nil {
			return
		}

		switch event.Type {
		case state.Errored:
			widget.showListError(formatError(event.Error))

			return
		case state.Bootstrapped:
			widget.bootstrapped++
		case state.Created, state.Updated:
			widget.storeResource(event.Resource, false)
		case state.Destroyed:
			widget.storeResource(event.Resource, true)
		case state.Noop:
			return
		}

		if widget.bootstrapped < vmWatchKinds {
			return
		}

		switch widget.level {
		case 0:
			widget.renderList()
		case 1:
			widget.renderDetail()
		}
	})
}

// storeResource files a watched resource into the map for its kind, or removes it when destroyed.
func (widget *VMsGrid) storeResource(res resource.Resource, destroyed bool) {
	id := res.Metadata().ID()

	switch typed := res.(type) {
	case *hypervisor.VirtualMachineSpec:
		storeOrDelete(widget.res.specs, id, typed, destroyed)
	case *hypervisor.VirtualMachineStatus:
		storeOrDelete(widget.res.statuses, id, typed, destroyed)
	case *hypervisor.VirtualMachineDomainStatus:
		storeOrDelete(widget.res.domains, id, typed, destroyed)
	case *hypervisor.VirtualMachineDiskStatus:
		storeOrDelete(widget.res.disks, id, typed, destroyed)
	case *hypervisor.VirtualMachinePowerOverride:
		storeOrDelete(widget.res.overrides, id, typed, destroyed)
	case *hypervisor.VirtualMachineLifecycle:
		if destroyed {
			widget.res.lifecycle = nil
		} else {
			widget.res.lifecycle = typed
		}
	}
}

// openDetail drills into the VM on the given list row.
func (widget *VMsGrid) openDetail(row int) {
	if row < 1 || row-1 >= len(widget.rows) {
		return
	}

	widget.selected = widget.rows[row-1].Name
	widget.level = 1
	widget.showDevices = false

	widget.logViewer.Reset()
	widget.renderDetail() // also starts the logs, once there are any

	widget.pages.SwitchToPage(pageVMDetail)
	widget.focusLevel()
}

// renderDetail rebuilds the detail level for the selected VM.
func (widget *VMsGrid) renderDetail() {
	if widget.selected == "" {
		return
	}

	widget.swapDetailPane()

	if widget.showDevices {
		widget.renderDeviceTable()
	} else {
		widget.renderDetailView()
	}

	if widget.active && !widget.logStreaming {
		widget.startLogs()
	}

	widget.renderLogLabel()
}

// swapDetailPane puts either the field view or the devices table in the top row.
func (widget *VMsGrid) swapDetailPane() {
	want := widget.topDetailPane()

	widget.detailWrapper.RemoveItem(widget.detailView)
	widget.detailWrapper.RemoveItem(widget.deviceTable)
	widget.detailWrapper.AddItem(want, 0, 0, 1, 1, 0, 0, false)
}

// renderDetailView renders the field summary of the selected VM.
func (widget *VMsGrid) renderDetailView() {
	title := widget.selected

	if status := widget.res.statuses[widget.selected]; status != nil {
		title = fmt.Sprintf("%s — %s / %s", widget.selected, status.TypedSpec().PowerState, status.TypedSpec().Stage)
	}

	hints := "i: devices, y: YAML, S/T/K/R: power, Tab: focus, Esc: back"
	if widget.message != "" {
		hints = widget.message
	}

	widget.detailView.SetTitle(fmt.Sprintf(" %s (%s) ", tview.Escape(title), hints))
	widget.detailView.SetText(formatVMDetail(widget.res, widget.selected))
	widget.detailView.ScrollToBeginning()
}

// renderDeviceTable renders the disks and interfaces of the selected VM.
func (widget *VMsGrid) renderDeviceTable() {
	previous, _ := widget.deviceTable.GetSelection()

	widget.deviceTable.Clear()

	for i, name := range vmDeviceHeader {
		widget.deviceTable.SetCell(0, i, headerCell(name))
	}

	devices := vmDeviceRows(widget.res, widget.selected)

	if len(devices) == 0 {
		widget.deviceTable.SetCell(1, 0, &tview.TableCell{
			Text:          "[gray]No devices configured.[-]",
			NotSelectable: true,
			Expansion:     1,
		})

		return
	}

	for i, cells := range devices {
		for column, text := range cells {
			widget.deviceTable.SetCell(i+1, column, &tview.TableCell{
				Text:      tview.Escape(text),
				Align:     tview.AlignLeft,
				Color:     tcell.ColorDefault,
				Expansion: vmDeviceColumnExpansion(column),
			})
		}
	}

	widget.deviceTable.Select(min(max(previous, 1), len(devices)), 0)
}

// vmDeviceColumnExpansion makes the name and the free-form detail columns absorb the width.
func vmDeviceColumnExpansion(column int) int {
	switch column {
	case 1:
		return 1
	case 5:
		return 3
	default:
		return 0
	}
}

// renderLogLabel keeps the log pane's label in sync with the selected VM.
func (widget *VMsGrid) renderLogLabel() {
	hints := []string{"/: filter"}

	if hint := vmLogsHint(widget.res.specs[widget.selected], widget.res.domains[widget.selected]); hint != "" {
		hints = append(hints, hint)
	}

	widget.logViewer.SetLabel(fmt.Sprintf("Console log: %s (%s)", widget.selected, strings.Join(hints, ", ")))
}

// startLogs points the log source at the serial console log of the selected VM.
func (widget *VMsGrid) startLogs() {
	if widget.selected == "" {
		return
	}

	// Hold off until there is a log to read; renderDetail comes back here on every watch update.
	if !vmLogsAvailable(widget.res.specs[widget.selected], widget.res.domains[widget.selected]) {
		widget.stopLogs()

		return
	}

	widget.logSession = widget.logSource.Start(widget.ctx, containerlogdata.Target{
		Node:      widget.selectedNode,
		Namespace: vmLogNamespace,
		Kind:      machine.LogKind_LOG_KIND_VM,
		ID:        widget.selected,
	})
	widget.logStreaming = true

	widget.renderLogLabel()
}

// stopLogs stops the log stream, if any.
func (widget *VMsGrid) stopLogs() {
	widget.logSource.Stop()
	widget.logStreaming = false
}

// writeLog writes a log line into the viewer, dropping lines of a stream already switched away from.
func (widget *VMsGrid) writeLog(data containerlogdata.Data) {
	if !widget.logStreaming || data.Session != widget.logSession {
		return
	}

	if data.Reset {
		follow := widget.logViewer.Follow()

		widget.logViewer.Reset()
		widget.logViewer.SetFollow(follow)

		return
	}

	widget.logViewer.WriteLog(data.Log, data.Error)
}

// openYAML opens the YAML of the VM the current level is showing: the one under the cursor on the
// list, the one drilled into on the detail.
func (widget *VMsGrid) openYAML() {
	name := widget.selected
	if widget.level == 0 {
		name = widget.selectedName()
	}

	if name == "" {
		return
	}

	widget.yamlName = name
	widget.yamlReturn = widget.level

	widget.showYAML(0)
}

// showYAML renders the resources of the VM opened by openYAML as YAML. kind selects which of them,
// and wraps around so that Tab cycles.
func (widget *VMsGrid) showYAML(kind int) {
	name := widget.yamlName

	sources := []struct {
		label string
		res   resource.Resource
	}{
		{"VirtualMachineSpec", lookupResource(widget.res.specs, name)},
		{"VirtualMachineStatus", lookupResource(widget.res.statuses, name)},
		{"VirtualMachineDomainStatus", lookupResource(widget.res.domains, name)},
		{"VirtualMachinePowerOverride", lookupResource(widget.res.overrides, name)},
	}

	widget.yamlKind = kind % len(sources)
	source := sources[widget.yamlKind]

	widget.yamlView.SetTitle(fmt.Sprintf(" %s / %s (Tab: next resource, Esc: back) ", tview.Escape(name), source.label))

	if source.res == nil {
		widget.yamlView.SetText("[gray]not available[-]")
	} else {
		widget.yamlView.SetText(resourceYAMLText(source.res))
	}

	widget.yamlView.ScrollToBeginning()

	widget.level = 2
	widget.pages.SwitchToPage(pageVMYAML)
	widget.focusLevel()
}
