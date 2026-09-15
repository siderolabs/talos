// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/pkg/dashboard/apidata"
	"github.com/siderolabs/talos/internal/pkg/dashboard/components"
	"github.com/siderolabs/talos/internal/pkg/dashboard/containerlogdata"
	"github.com/siderolabs/talos/internal/pkg/dashboard/utils"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

const (
	pageContainerList   = "container-list"
	pageContainerDetail = "container-detail"
	pageContainerYAML   = "container-yaml"

	// containerDetailRows is the height of the detail pane above the logs: the worst-case field
	// count of formatContainerDetail plus the border, so its height is known rather than measured.
	containerDetailRows = 10

	// containerWatchKinds is how many resource kinds the screen watches, and therefore how many
	// bootstrap events it waits for before it starts rendering.
	containerWatchKinds = 3
)

// ContainersGrid implements the containers screen: a list of the containers declared via
// ContainerConfig, drilling down into a detail pane with live logs, and into the raw YAML.
//
// The list is driven by COSI rather than by the Containers API: ContainerStatus carries the health,
// the unmet dependencies and the restart history that the API has no notion of. Metrics are the
// one thing COSI cannot provide, and those are joined in from the polled Stats API.
type ContainersGrid struct {
	tview.Grid

	app       *tview.Application
	dashboard *Dashboard
	ctx       context.Context //nolint:containedctx // used to stop/start watches

	pages *tview.Pages
	level int // 0=list, 1=detail, 2=yaml

	// Level 0: container list
	listWrapper  *tview.Grid
	table        *tview.Table
	summary      *tview.TextView
	filterInput  *tview.InputField
	filterActive bool
	filterText   string
	sortBy       containerSort

	// Watched resources, keyed by resource ID.
	statuses     map[string]*containers.ContainerStatus
	specs        map[string]*containers.ContainerSpec
	instances    map[string]*containers.ContainerInstanceStatus
	watchCancel  context.CancelFunc
	bootstrapped int

	// Metrics joined in from the API data source.
	stats    map[string]*machine.Stat
	cpuDiff  map[string]uint64
	interval time.Duration

	// rows are the containers currently displayed, in display order (row-1 indexed).
	rows []containerRow

	// nsIndex selects the containerd namespace shown; see containerNamespaces. Everything below
	// is only used for the namespaces that COSI knows nothing about.
	nsIndex     int
	apiCancel   context.CancelFunc
	apiInfos    []*machine.ContainerInfo
	apiStats    map[string]*machine.Stat
	apiCPU      map[string]uint64
	apiPercent  map[string]float64
	apiPolledAt time.Time
	apiRows     []apiContainerRow

	// Level 1: detail and logs
	detailWrapper *tview.Grid
	detailView    *tview.TextView
	instanceTable *tview.Table
	// instanceRows are the instances currently in instanceTable, in display order (row-1 indexed).
	instanceRows  []*containers.ContainerInstanceStatus
	showInstances bool
	logViewer     *components.LogViewer
	logSource     *containerlogdata.Source
	logTarget     containerlogdata.Target
	selected      string
	selectedAPI   apiContainerRow

	// Level 2: YAML
	yamlView *tview.TextView
	yamlKind int
	// yamlFrom is the level the YAML view was opened from, and the one Esc returns to.
	yamlFrom int

	selectedNode string
	active       bool
}

// NewContainersGrid initializes ContainersGrid.
func NewContainersGrid(ctx context.Context, dashboard *Dashboard) *ContainersGrid {
	widget := &ContainersGrid{
		Grid:      *tview.NewGrid(),
		app:       dashboard.app,
		dashboard: dashboard,
		ctx:       ctx,
		statuses:  map[string]*containers.ContainerStatus{},
		specs:     map[string]*containers.ContainerSpec{},
		instances: map[string]*containers.ContainerInstanceStatus{},
		logSource: containerlogdata.NewSource(dashboard.cli),
	}

	widget.SetRows(0).SetColumns(0)

	widget.initList()
	widget.initDetail()
	widget.initYAML()

	widget.pages = tview.NewPages()
	widget.pages.AddPage(pageContainerList, widget.listWrapper, true, true)
	widget.pages.AddPage(pageContainerDetail, widget.detailWrapper, true, false)
	widget.pages.AddPage(pageContainerYAML, widget.yamlView, true, false)

	widget.AddItem(widget.pages, 0, 0, 1, 1, 0, 0, true)

	widget.SetInputCapture(widget.handleKey)

	widget.initTableHeader()
	widget.renderList()

	go widget.runLogConsumer(ctx)

	return widget
}

func (widget *ContainersGrid) initList() {
	widget.table = tview.NewTable()
	widget.table.SetBorder(true)
	widget.table.SetFixed(1, 0)
	widget.table.SetSelectable(true, false)
	widget.table.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))
	widget.table.SetSelectedFunc(func(row, _ int) {
		widget.openDetail(row)
	})

	widget.summary = tview.NewTextView()
	widget.summary.SetDynamicColors(true)
	widget.summary.SetBorderPadding(0, 0, 1, 1)

	widget.filterInput = tview.NewInputField()
	widget.filterInput.SetLabel("filter: ")
	widget.filterInput.SetLabelColor(tcell.ColorYellow)
	widget.filterInput.SetFieldBackgroundColor(tcell.ColorDefault)
	widget.filterInput.SetChangedFunc(func(text string) {
		widget.filterText = text
		widget.renderList()
	})
	widget.filterInput.SetDoneFunc(func(key tcell.Key) {
		widget.deactivateFilter(key == tcell.KeyEscape)
	})

	widget.listWrapper = tview.NewGrid()
	widget.listWrapper.SetRows(0, 1).SetColumns(0)
	widget.listWrapper.AddItem(widget.table, 0, 0, 1, 1, 0, 0, true)
	widget.listWrapper.AddItem(widget.summary, 1, 0, 1, 1, 0, 0, false)
}

func (widget *ContainersGrid) initDetail() {
	widget.detailView = tview.NewTextView()
	widget.detailView.SetDynamicColors(true)
	widget.detailView.SetBorder(true)

	widget.instanceTable = tview.NewTable()
	widget.instanceTable.SetBorder(true)
	widget.instanceTable.SetTitle(" Instances (i: back to details) ")
	widget.instanceTable.SetFixed(1, 0)
	widget.instanceTable.SetSelectable(true, false)
	widget.instanceTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))

	widget.logViewer = components.NewLogViewer(widget.app)

	widget.detailWrapper = tview.NewGrid()
	widget.detailWrapper.SetRows(containerDetailRows, 0).SetColumns(0)
	widget.detailWrapper.AddItem(widget.detailView, 0, 0, 1, 1, 0, 0, false)
	widget.detailWrapper.AddItem(widget.logViewer, 1, 0, 1, 1, 0, 0, true)
}

func (widget *ContainersGrid) initYAML() {
	widget.yamlView = tview.NewTextView()
	widget.yamlView.SetBorder(true)
	widget.yamlView.SetScrollable(true)
	widget.yamlView.SetDynamicColors(true)
}

// onScreenSelect implements the screenSelectListener interface.
func (widget *ContainersGrid) onScreenSelect(active bool) {
	widget.active = active

	if !active {
		// Nothing on this screen is visible, so stop both streams; they are restarted on the next
		// activation with the state they had.
		widget.stopSources()
		widget.logSource.Stop()
		widget.deactivateFilter(false)

		return
	}

	widget.startSources()

	// logTarget is set for as long as a container is drilled into, which includes the YAML level
	// above it: re-entering the screen there resumes the stream the operator left running.
	if widget.logTarget.ID != "" {
		widget.logSource.Start(widget.ctx, widget.logTarget)
	}

	widget.focusLevel()
}

// OnNodeSelect implements the NodeSelectListener interface.
func (widget *ContainersGrid) OnNodeSelect(node string) {
	if node == widget.selectedNode {
		return
	}

	widget.selectedNode = node

	// Everything on the screen belongs to the node that was selected, down to the container the
	// user drilled into, so drop back to the list rather than showing another node's container
	// under the same name.
	widget.stopSources()
	widget.logSource.Stop()
	widget.logTarget = containerlogdata.Target{}
	widget.deactivateFilter(false)

	widget.statuses = map[string]*containers.ContainerStatus{}
	widget.specs = map[string]*containers.ContainerSpec{}
	widget.instances = map[string]*containers.ContainerInstanceStatus{}
	widget.stats = nil
	widget.cpuDiff = nil
	widget.apiInfos = nil
	widget.apiStats = nil
	widget.apiPercent = nil
	widget.apiRows = nil
	widget.selected = ""
	widget.level = 0
	widget.showInstances = false

	widget.pages.SwitchToPage(pageContainerList)
	widget.logViewer.Reset()
	widget.renderList()

	if widget.active {
		widget.startSources()
		widget.focusLevel()
	}
}

// OnAPIDataChange implements the APIDataListener interface.
func (widget *ContainersGrid) OnAPIDataChange(node string, data *apidata.Data) {
	nodeData := data.Nodes[node]
	if nodeData == nil {
		return
	}

	widget.interval = data.Interval
	widget.cpuDiff = nodeData.ContainerCPUDiff
	widget.stats = map[string]*machine.Stat{}

	for _, stat := range nodeData.ContainerStats.GetStats() {
		widget.stats[stat.Id] = stat
	}

	if widget.level == 0 {
		widget.renderList()
	}
}

// focusLevel gives the focus to the primitive of the current level.
func (widget *ContainersGrid) focusLevel() {
	switch widget.level {
	case 0:
		widget.app.SetFocus(widget.table)
	case 1:
		widget.app.SetFocus(widget.logViewer)
	case 2:
		widget.app.SetFocus(widget.yamlView)
	}
}

// handleKey handles the screen-level shortcuts.
//
// It runs before the focused primitive sees the event, so it deliberately passes everything
// through while a text input has the focus: the filter fields own printable characters.
//
//nolint:gocyclo,cyclop
func (widget *ContainersGrid) handleKey(event *tcell.EventKey) *tcell.EventKey {
	if _, isInput := widget.app.GetFocus().(*tview.InputField); isInput {
		return event
	}

	if event.Key() == tcell.KeyEscape {
		widget.goBack()

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
	case 'n':
		if widget.level == 0 {
			widget.cycleNamespace()

			return nil
		}
	case 'y':
		if widget.level <= 1 && widget.talosMode() {
			widget.showYAML(0)

			return nil
		}
	case 'i':
		if widget.level == 1 && widget.talosMode() {
			widget.toggleInstances()

			return nil
		}
	case 'f':
		if widget.level == 1 {
			widget.logViewer.SetFollow(!widget.logViewer.Follow())
			widget.renderLogLabel()

			return nil
		}
	case 'w':
		if widget.level == 1 {
			widget.logViewer.SetWrap(!widget.logViewer.Wrap())
			widget.renderLogLabel()

			return nil
		}
	case 'g':
		// The detail level has no scrollable of its own to jump: the focused pane handles these.
		if widget.level != 1 {
			widget.scrollToStart()

			return nil
		}
	case 'G':
		if widget.level != 1 {
			widget.scrollToEnd()

			return nil
		}
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

// toggleInstances swaps the detail pane between the field summary and the instance history,
// carrying the focus over: swapping the pane out from under the cursor would otherwise leave the
// screen with nothing focused.
func (widget *ContainersGrid) toggleInstances() {
	focused := widget.app.GetFocus()
	topWasFocused := focused == tview.Primitive(widget.detailView) || focused == tview.Primitive(widget.instanceTable)

	widget.showInstances = !widget.showInstances

	widget.renderDetail(true)

	if topWasFocused {
		widget.app.SetFocus(widget.topDetailPane())
	}
}

// topDetailPane returns the primitive currently occupying the top row of the detail level.
func (widget *ContainersGrid) topDetailPane() tview.Primitive { //nolint:ireturn
	if widget.showInstances {
		return widget.instanceTable
	}

	return widget.detailView
}

// scrollToStart jumps to the top of whatever the current level shows.
func (widget *ContainersGrid) scrollToStart() {
	switch widget.level {
	case 0:
		if widget.table.GetRowCount() > 1 {
			widget.table.SetOffset(0, 0)
			widget.table.Select(1, 0)
		}
	case 2:
		widget.yamlView.ScrollToBeginning()
	}
}

// scrollToEnd jumps to the bottom of whatever the current level shows.
func (widget *ContainersGrid) scrollToEnd() {
	switch widget.level {
	case 0:
		if rowCount := widget.table.GetRowCount(); rowCount > 1 {
			widget.table.Select(rowCount-1, 0)
		}
	case 2:
		widget.yamlView.ScrollToEnd()
	}
}

// toggleDetailFocus moves the focus between the top pane and the logs, so that both can be
// scrolled without leaving the detail level.
func (widget *ContainersGrid) toggleDetailFocus() {
	top := widget.topDetailPane()

	if widget.app.GetFocus() == top {
		widget.app.SetFocus(widget.logViewer)

		return
	}

	widget.app.SetFocus(top)
}

// activateFilter shows the filter input below the list.
func (widget *ContainersGrid) activateFilter() {
	if widget.filterActive {
		return
	}

	widget.filterActive = true
	widget.filterInput.SetText(widget.filterText)
	widget.listWrapper.SetRows(0, 1, 1)
	widget.listWrapper.AddItem(widget.filterInput, 2, 0, 1, 1, 0, 0, true)
	widget.app.SetFocus(widget.filterInput)
}

// deactivateFilter hides the filter input. If clearText is true, the filter is also cleared.
func (widget *ContainersGrid) deactivateFilter(clearText bool) {
	if !widget.filterActive {
		if clearText && widget.filterText != "" {
			widget.filterText = ""
			widget.filterInput.SetText("")
			widget.renderList()
		}

		return
	}

	if clearText {
		widget.filterText = ""
		widget.filterInput.SetText("")
	}

	widget.filterActive = false
	widget.listWrapper.RemoveItem(widget.filterInput)
	widget.listWrapper.SetRows(0, 1)

	if clearText {
		widget.renderList()
	}

	if widget.active && widget.level == 0 {
		widget.app.SetFocus(widget.table)
	}
}

// goBack returns to the previous navigation level.
func (widget *ContainersGrid) goBack() {
	switch widget.level {
	case 2:
		// The YAML view is reachable from the list as well as from the detail, and dropping into a
		// detail level that was never rendered would show a stale pane with no log stream.
		widget.level = widget.yamlFrom

		if widget.yamlFrom == 1 {
			widget.pages.SwitchToPage(pageContainerDetail)
		} else {
			widget.selected = ""
			widget.pages.SwitchToPage(pageContainerList)
		}
	case 1:
		widget.logSource.Stop()
		widget.logTarget = containerlogdata.Target{}
		widget.logViewer.Reset()

		widget.level = 0
		widget.selected = ""
		widget.showInstances = false

		widget.pages.SwitchToPage(pageContainerList)
	default:
		if widget.filterText != "" {
			widget.deactivateFilter(true)
		}
	}

	widget.focusLevel()
}

// initTableHeader sets the header row of the container list.
func (widget *ContainersGrid) initTableHeader() {
	widget.table.Clear()

	columns := []string{"NAMESPACE", "ID", "IMAGE", "PID", "STATUS", "CPU", "MEM"}
	if widget.talosMode() {
		columns = []string{"NAME", "STATE", "HEALTH", "RESTARTS", "PID", "CPU", "MEM", "IMAGE"}
	}

	for i, name := range columns {
		widget.table.SetCell(0, i, headerCell(name))
	}
}

// renderList rebuilds the container list from the watched resources and the polled metrics.
func (widget *ContainersGrid) renderList() {
	widget.table.SetTitle(widget.listTitle())

	if !widget.talosMode() {
		widget.renderAPIList()

		return
	}

	// Remember the selected container rather than the row: sorting and live updates both move
	// rows around underneath the cursor.
	previous := widget.selectedName()

	widget.rows = buildContainerRows(widget.statuses, widget.stats, widget.cpuDiff, widget.interval, widget.filterText, widget.sortBy)

	widget.initTableHeader()

	for i, row := range widget.rows {
		color := healthColor(row.Health)

		for column, text := range []string{
			tview.Escape(row.Name),
			colored(color, tview.Escape(row.State)),
			colored(color, row.Health.String()),
			strconv.FormatUint(row.Restarts, 10),
			formatPID(row.PID),
			formatCPUPercent(row.CPUPercent),
			formatMemory(row),
			tview.Escape(row.Image),
		} {
			widget.table.SetCell(i+1, column, &tview.TableCell{
				Text:      text,
				Align:     tview.AlignLeft,
				Color:     tcell.ColorDefault,
				Expansion: expansionForColumn(column),
			})
		}
	}

	widget.summary.SetText(containerHealthSummary(widget.rows))

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

// listTitle names the namespace shown and the shortcuts that apply to it.
func (widget *ContainersGrid) listTitle() string {
	if !widget.talosMode() {
		return fmt.Sprintf(" Containers: %s (Enter: logs, /: filter, n: namespace, Esc: back) ", widget.namespaceLabel())
	}

	return fmt.Sprintf(
		" Containers: %s — by %s (Enter: details, /: filter, s: sort, n: namespace, y: YAML) ",
		widget.namespaceLabel(), widget.sortBy,
	)
}

// expansionForColumn returns the expansion factor of a list column: only the name and the image
// grow, since the image reference is the one field that can be arbitrarily long.
func expansionForColumn(column int) int {
	switch column {
	case 0:
		return 1
	case 7:
		return 3
	default:
		return 0
	}
}

// emptyListText explains an empty list, which on a node with no ContainerConfig documents is the
// normal state rather than a problem.
func (widget *ContainersGrid) emptyListText() string {
	switch {
	case widget.filterText != "":
		return "No containers match the filter."
	case widget.bootstrapped < containerWatchKinds:
		return "Loading..."
	default:
		return "No containers declared. Add a ContainerConfig document to the machine configuration."
	}
}

// restoreSelection selects the row of the named container, falling back to the first row when it
// is gone.
func (widget *ContainersGrid) restoreSelection(name string) {
	if name != "" {
		if index := slices.IndexFunc(widget.rows, func(row containerRow) bool { return row.Name == name }); index >= 0 {
			widget.table.Select(index+1, 0)

			return
		}
	}

	widget.table.SetOffset(0, 0)
	widget.table.Select(1, 0)
}

// selectedName returns the container the cursor is on, or an empty string when the list is empty.
func (widget *ContainersGrid) selectedName() string {
	row, _ := widget.table.GetSelection()

	if row < 1 || row-1 >= len(widget.rows) {
		return ""
	}

	return widget.rows[row-1].Name
}

// showListError replaces the list with an error message.
func (widget *ContainersGrid) showListError(msg string) {
	widget.initTableHeader()
	widget.table.SetCell(1, 0, &tview.TableCell{
		Text:          fmt.Sprintf("[red]%s[-]", tview.Escape(msg)),
		NotSelectable: true,
		Expansion:     1,
	})
	widget.summary.SetText("")
}

// startSources starts whichever data source backs the namespace currently shown.
func (widget *ContainersGrid) startSources() {
	if widget.talosMode() {
		widget.startWatch()

		return
	}

	widget.startAPIPoll()
}

// stopSources stops both data sources.
func (widget *ContainersGrid) stopSources() {
	widget.stopWatch()
	widget.stopAPIPoll()
}

// startWatch begins watching the container resources of the selected node.
func (widget *ContainersGrid) startWatch() {
	if widget.watchCancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(widget.ctx)
	widget.watchCancel = cancel

	// The node is captured here, on the UI goroutine: OnNodeSelect writes it, and a watch that
	// has been cancelled but not yet returned must not read it.
	go widget.runWatch(ctx, widget.selectedNode)
}

// stopWatch cancels a running watch, if any, and drops what it had gathered.
//
// The next watch bootstraps with full contents, so keeping the maps would only retain the
// containers destroyed while the screen was away: their Destroyed events are never delivered.
func (widget *ContainersGrid) stopWatch() {
	if widget.watchCancel != nil {
		widget.watchCancel()
		widget.watchCancel = nil
	}

	widget.bootstrapped = 0

	clear(widget.statuses)
	clear(widget.specs)
	clear(widget.instances)
}

// runWatch runs the watch event loop for all the container resource kinds at once.
//
// A single event channel keeps the three kinds in step: the list joins them, so rendering on every
// event from any of them is both correct and simpler than reconciling three separate streams.
func (widget *ContainersGrid) runWatch(ctx context.Context, node string) {
	nodeCtx := utils.NodeContext(ctx, node)

	eventCh := make(chan state.Event)

	for _, resourceType := range []resource.Type{
		containers.ContainerStatusType,
		containers.ContainerSpecType,
		containers.ContainerInstanceStatusType,
	} {
		md := resource.NewMetadata(containers.NamespaceName, resourceType, "", resource.VersionUndefined)

		if err := widget.dashboard.cli.COSI.WatchKind(nodeCtx, &md, eventCh, state.WithBootstrapContents(true)); err != nil {
			if isCanceled(err) {
				return
			}

			widget.app.QueueUpdate(func() {
				widget.showListError(formatError(err))
			})

			return
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-eventCh:
			widget.handleWatchEvent(event)
		}
	}
}

// handleWatchEvent applies a single watch event to the widget state, on the UI goroutine.
func (widget *ContainersGrid) handleWatchEvent(event state.Event) {
	widget.app.QueueUpdate(func() {
		switch event.Type {
		case state.Errored:
			if !isCanceled(event.Error) {
				widget.showListError(formatError(event.Error))
			}

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

		if widget.bootstrapped < containerWatchKinds {
			return
		}

		if widget.level == 0 {
			widget.renderList()
		} else {
			// A watch tick is not a navigation: leave the scroll offset and the instance-table
			// cursor where the operator put them.
			widget.renderDetail(false)
		}
	})
}

// storeResource files a watched resource into the map for its kind, or removes it when destroyed.
func (widget *ContainersGrid) storeResource(res resource.Resource, destroyed bool) {
	id := res.Metadata().ID()

	switch typed := res.(type) {
	case *containers.ContainerStatus:
		storeOrDelete(widget.statuses, id, typed, destroyed)
	case *containers.ContainerSpec:
		storeOrDelete(widget.specs, id, typed, destroyed)
	case *containers.ContainerInstanceStatus:
		storeOrDelete(widget.instances, id, typed, destroyed)
	}
}

func storeOrDelete[T any](target map[string]T, id string, value T, destroyed bool) {
	if destroyed {
		delete(target, id)

		return
	}

	target[id] = value
}

// containerInstances returns the retained instances of a container, newest generation first.
func (widget *ContainersGrid) containerInstances(name string) []*containers.ContainerInstanceStatus {
	result := make([]*containers.ContainerInstanceStatus, 0, len(widget.instances))

	for _, instance := range widget.instances {
		if instance.TypedSpec().ContainerID == name {
			result = append(result, instance)
		}
	}

	slices.SortFunc(result, func(a, b *containers.ContainerInstanceStatus) int {
		return int(b.TypedSpec().Generation) - int(a.TypedSpec().Generation)
	})

	return result
}

// openDetail drills into the container on the given list row.
func (widget *ContainersGrid) openDetail(row int) {
	if row < 1 {
		return
	}

	if !widget.talosMode() {
		if row-1 >= len(widget.apiRows) {
			return
		}

		widget.selectedAPI = widget.apiRows[row-1]
		widget.selected = widget.selectedAPI.ID
	} else {
		if row-1 >= len(widget.rows) {
			return
		}

		widget.selected = widget.rows[row-1].Name
		widget.selectedAPI = apiContainerRow{}
	}

	widget.level = 1
	widget.showInstances = false
	// The instance table still holds the previous container's rows; dropping them keeps its
	// selection from being restored onto a generation that belongs to something else.
	widget.instanceRows = nil

	widget.logViewer.Reset()
	widget.renderDetail(true)

	widget.pages.SwitchToPage(pageContainerDetail)
	widget.startLogs()
	widget.focusLevel()
}

// renderDetail rebuilds the detail level for the selected container.
//
// resetView says whether this render is a navigation, and therefore whether the panes may move
// back to the top; a live update passes false and leaves them where they are.
func (widget *ContainersGrid) renderDetail(resetView bool) {
	if widget.selected == "" {
		return
	}

	widget.swapDetailPane()

	if !widget.talosMode() {
		widget.detailView.SetTitle(apiDetailTitle(widget.selected))
		widget.detailView.SetText(formatAPIContainerDetail(widget.selectedAPI))

		if resetView {
			widget.detailView.ScrollToBeginning()
		}

		widget.renderLogLabel()

		return
	}

	instances := widget.containerInstances(widget.selected)

	if widget.showInstances {
		widget.renderInstanceTable(instances)
	} else {
		widget.renderDetailView(instances, resetView)
	}

	widget.renderLogLabel()
}

// swapDetailPane puts either the field view or the instance table in the top row.
func (widget *ContainersGrid) swapDetailPane() {
	want := widget.topDetailPane()

	widget.detailWrapper.RemoveItem(widget.detailView)
	widget.detailWrapper.RemoveItem(widget.instanceTable)
	widget.detailWrapper.AddItem(want, 0, 0, 1, 1, 0, 0, false)
}

// renderDetailView renders the field summary of the selected container.
func (widget *ContainersGrid) renderDetailView(instances []*containers.ContainerInstanceStatus, resetView bool) {
	status, spec := widget.statuses[widget.selected], widget.specs[widget.selected]

	var newest *containers.ContainerInstanceStatus
	if len(instances) > 0 {
		newest = instances[0]
	}

	widget.detailView.SetTitle(detailTitle(widget.selected, status))
	widget.detailView.SetText(formatContainerDetail(status, spec, newest, time.Now()))

	if resetView {
		widget.detailView.ScrollToBeginning()
	}
}

// renderInstanceTable renders the retained execution attempts of the selected container.
func (widget *ContainersGrid) renderInstanceTable(instances []*containers.ContainerInstanceStatus) {
	// Remember the generation rather than the row: a new instance is prepended to the list, which
	// would otherwise move every row under the cursor.
	previous := widget.selectedGeneration()

	widget.instanceRows = instances

	widget.instanceTable.Clear()

	for i, name := range []string{"GEN", "PHASE", "PID", "EXIT", "STARTED", "FINISHED", "ERROR"} {
		widget.instanceTable.SetCell(0, i, headerCell(name))
	}

	if len(instances) == 0 {
		widget.instanceTable.SetCell(1, 0, &tview.TableCell{
			Text:          "[gray]No instances recorded.[-]",
			NotSelectable: true,
			Expansion:     1,
		})

		return
	}

	for i, instance := range instances {
		for column, text := range xslices.Map(instanceRowCells(instance.TypedSpec()), tview.Escape) {
			widget.instanceTable.SetCell(i+1, column, &tview.TableCell{
				Text:      text,
				Align:     tview.AlignLeft,
				Color:     tcell.ColorDefault,
				Expansion: min(column, 1),
			})
		}
	}

	widget.restoreInstanceSelection(previous)
}

// selectedGeneration returns the generation of the instance the cursor is on, or none when the
// table is empty.
func (widget *ContainersGrid) selectedGeneration() optional.Optional[uint64] {
	row, _ := widget.instanceTable.GetSelection()

	if row < 1 || row-1 >= len(widget.instanceRows) {
		return optional.None[uint64]()
	}

	return optional.Some(widget.instanceRows[row-1].TypedSpec().Generation)
}

// restoreInstanceSelection selects the row of the given generation, falling back to the first row
// when it has been retired.
func (widget *ContainersGrid) restoreInstanceSelection(generation optional.Optional[uint64]) {
	if value, ok := generation.Get(); ok {
		if index := slices.IndexFunc(widget.instanceRows, func(instance *containers.ContainerInstanceStatus) bool {
			return instance.TypedSpec().Generation == value
		}); index >= 0 {
			widget.instanceTable.Select(index+1, 0)

			return
		}
	}

	widget.instanceTable.SetOffset(0, 0)
	widget.instanceTable.Select(1, 0)
}

// renderLogLabel keeps the log pane's label in sync with the container it shows and the toggles
// currently in effect.
func (widget *ContainersGrid) renderLogLabel() {
	hints := []string{"/: filter"}

	if widget.logViewer.Follow() {
		hints = append(hints, "f: following")
	} else {
		hints = append(hints, "f: frozen")
	}

	if !widget.logViewer.Wrap() {
		hints = append(hints, "w: nowrap")
	}

	widget.logViewer.SetLabel(logLabel(widget.selected, hints))
}

// startLogs points the log source at the selected container.
func (widget *ContainersGrid) startLogs() {
	if widget.selected == "" {
		return
	}

	target := containerNamespaces[widget.nsIndex]

	id := widget.selected
	if !widget.talosMode() {
		id = widget.selectedAPI.LogID
	}

	widget.logTarget = containerlogdata.Target{
		Node:      widget.selectedNode,
		Namespace: target.namespace,
		Driver:    target.driver,
		ID:        id,
	}

	widget.logSource.Start(widget.ctx, widget.logTarget)
}

// runLogConsumer drains the container log source into the log viewer.
//
// The screen owns its log stream rather than routing it through the dashboard's data handler,
// because the stream follows the selected container rather than the selected node. The batching
// mirrors what the dashboard does for node logs: a burst of lines produces one queued update
// instead of one per line.
func (widget *ContainersGrid) runLogConsumer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-widget.logSource.LogCh:
			batch := []containerlogdata.Data{data}

		drain:
			for {
				select {
				case more := <-widget.logSource.LogCh:
					batch = append(batch, more)
				default:
					break drain
				}
			}

			widget.app.QueueUpdate(func() {
				for _, entry := range batch {
					widget.writeLog(entry)
				}
			})
		}
	}
}

// writeLog writes a log line into the log viewer, dropping the lines that belong to a stream the
// screen has already switched away from: cancelling a stream does not retract the lines already
// queued behind it.
//
// Lines that arrive while the YAML view is open are still written: the viewer buffers them, so
// they are there when the operator comes back rather than a gap in the scrollback.
func (widget *ContainersGrid) writeLog(data containerlogdata.Data) {
	if data.Target != widget.logTarget || widget.level == 0 {
		return
	}

	widget.logViewer.WriteLog(data.Log, data.Error)
}

// showYAML renders the selected container's resources as YAML. kind selects which of them, and
// wraps around so that Tab cycles.
func (widget *ContainersGrid) showYAML(kind int) {
	name := widget.selected
	if widget.level == 0 {
		name = widget.selectedName()
	}

	if name == "" {
		return
	}

	instances := widget.containerInstances(name)

	sources := []struct {
		label string
		res   resource.Resource
	}{
		{"ContainerStatus", lookupResource(widget.statuses, name)},
		{"ContainerSpec", lookupResource(widget.specs, name)},
		{"ContainerInstanceStatus", newestInstance(instances)},
	}

	widget.yamlKind = kind % len(sources)
	source := sources[widget.yamlKind]

	widget.yamlView.SetTitle(yamlTitle(name, source.label))

	if source.res == nil {
		widget.yamlView.SetText("[gray]not available[-]")
	} else {
		widget.yamlView.SetText(resourceYAMLText(source.res))
	}

	widget.yamlView.ScrollToBeginning()

	widget.selected = name

	// Tab cycles the resource without leaving the level, so the origin is only recorded on entry.
	if widget.level != 2 {
		widget.yamlFrom = widget.level
	}

	widget.level = 2
	widget.pages.SwitchToPage(pageContainerYAML)
	widget.focusLevel()
}

// isCanceled reports whether err is the screen shutting a stream down rather than a failure the
// operator needs to see: switching node, namespace or screen cancels the context out from under a
// call in flight, and its error would otherwise wipe the list that was just rendered.
func isCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled
}

// lookupResource returns a watched resource as a resource.Resource, or a nil interface when it is
// not present — a missing map entry would otherwise yield a non-nil interface holding a nil
// pointer.
func lookupResource[T resource.Resource](target map[string]T, id string) resource.Resource { //nolint:ireturn
	res, ok := target[id]
	if !ok {
		return nil
	}

	return res
}

func newestInstance(instances []*containers.ContainerInstanceStatus) resource.Resource { //nolint:ireturn
	if len(instances) == 0 {
		return nil
	}

	return instances[0]
}
