// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	yaml "go.yaml.in/yaml/v4"
	"google.golang.org/grpc/codes"
	"k8s.io/client-go/util/jsonpath"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

const (
	pageResourceTypes = "resource-types"
	pageResourceList  = "resource-list"
	pageResourceYAML  = "resource-yaml"
)

// ResourceExplorerGrid implements the resource browser screen.
type ResourceExplorerGrid struct {
	tview.Grid

	app       *tview.Application
	dashboard *Dashboard
	ctx       context.Context //nolint:containedctx // used to stop/start watches

	pages *tview.Pages
	level int // 0=types, 1=instances, 2=yaml

	// Level 0: resource type list
	typesWrapper *tview.Grid // contains typesTable + optional filterInput
	typesTable   *tview.Table
	filterInput  *tview.InputField
	filterActive bool
	filterText   string
	resourceDefs []*meta.ResourceDefinition

	// Level 1: resource instance list
	resourceTable      *tview.Table
	selectedRD         *meta.ResourceDefinition
	resources          map[string]resource.Resource
	sortedIDs          []string // sorted IDs matching table rows (row-1 indexed)
	watchCancel        context.CancelFunc
	dynamicColumnNames []string
	dynamicColumns     []func(any) (string, error)

	// Level 2: YAML detail
	yamlView *tview.TextView

	selectedNode string
	active       bool
}

// NewResourceExplorerGrid initializes ResourceExplorerGrid.
func NewResourceExplorerGrid(ctx context.Context, dashboard *Dashboard) *ResourceExplorerGrid {
	widget := &ResourceExplorerGrid{
		Grid:      *tview.NewGrid(),
		app:       dashboard.app,
		dashboard: dashboard,
		ctx:       ctx,
		resources: make(map[string]resource.Resource),
	}

	widget.SetRows(0).SetColumns(0)

	// Level 0: resource types table
	widget.typesTable = tview.NewTable()
	widget.typesTable.SetBorder(true).
		SetTitle(" Resource Types (Enter: select, /: filter) ")
	widget.typesTable.SetFixed(1, 0)
	widget.typesTable.SetSelectable(true, false)
	widget.typesTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))
	widget.typesTable.SetSelectedFunc(func(row, _ int) {
		widget.selectResourceType(row)
	})
	widget.typesTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == '/' {
			widget.activateFilter()

			return nil
		}

		return event
	})

	// Filter input shown below the types table when '/' is pressed
	widget.filterInput = tview.NewInputField()
	widget.filterInput.SetLabel("filter: ")
	widget.filterInput.SetLabelColor(tcell.ColorYellow)
	widget.filterInput.SetFieldBackgroundColor(tcell.ColorDefault)
	widget.filterInput.SetChangedFunc(func(text string) {
		widget.filterText = text
		widget.renderTypesTable()
	})
	widget.filterInput.SetDoneFunc(func(key tcell.Key) {
		// Esc clears the filter; Enter keeps the filtered view
		widget.deactivateFilter(key == tcell.KeyEscape)
	})

	// Wrapper grid: table fills row 0; filter input occupies row 1 when active
	widget.typesWrapper = tview.NewGrid()
	widget.typesWrapper.SetRows(0).SetColumns(0)
	widget.typesWrapper.AddItem(widget.typesTable, 0, 0, 1, 1, 0, 0, true)

	// Level 1: resource instances table
	widget.resourceTable = tview.NewTable()
	widget.resourceTable.SetBorder(true)
	widget.resourceTable.SetFixed(1, 0)
	widget.resourceTable.SetSelectable(true, false)
	widget.resourceTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))
	widget.resourceTable.SetSelectedFunc(func(row, _ int) {
		widget.selectResource(row)
	})
	widget.resourceTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			widget.goBack()

			return nil
		}

		return event
	})

	// Level 2: YAML detail view
	widget.yamlView = tview.NewTextView()
	widget.yamlView.SetBorder(true)
	widget.yamlView.SetScrollable(true)
	widget.yamlView.SetDynamicColors(true)
	widget.yamlView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			widget.goBack()

			return nil
		}

		return event
	})

	// Pages to manage which level is visible
	widget.pages = tview.NewPages()
	widget.pages.AddPage(pageResourceTypes, widget.typesWrapper, true, true)
	widget.pages.AddPage(pageResourceList, widget.resourceTable, true, false)
	widget.pages.AddPage(pageResourceYAML, widget.yamlView, true, false)

	widget.AddItem(widget.pages, 0, 0, 1, 1, 0, 0, true)

	widget.initTypesTableHeader()

	return widget
}

// onScreenSelect implements the screenSelectListener interface.
func (widget *ResourceExplorerGrid) onScreenSelect(active bool) {
	widget.active = active

	if active {
		switch widget.level {
		case 0:
			if widget.resourceDefs == nil {
				widget.loadResourceTypes()
			}

			widget.app.SetFocus(widget.typesTable)
		case 1:
			// Restart watch if it stopped when screen was hidden
			if widget.watchCancel == nil && widget.selectedRD != nil {
				widget.startResourceWatch(widget.selectedRD)
			}

			widget.app.SetFocus(widget.resourceTable)
		case 2:
			widget.app.SetFocus(widget.yamlView)
		}
	} else {
		// Stop watch while screen is hidden; it will be restarted on next activation
		widget.stopResourceWatch()
		widget.deactivateFilter(true)
	}
}

// OnNodeSelect implements the NodeSelectListener interface.
func (widget *ResourceExplorerGrid) OnNodeSelect(node string) {
	if node == widget.selectedNode {
		return
	}

	widget.selectedNode = node
	widget.stopResourceWatch()
	widget.deactivateFilter(true)

	switch widget.level {
	case 0:
		// Reload resource types for the new node
		widget.resourceDefs = nil
		widget.initTypesTableHeader()

		if widget.active {
			widget.loadResourceTypes()
		}
	case 1:
		// Stay on the resource list but reload for new node
		widget.resources = make(map[string]resource.Resource)
		widget.sortedIDs = nil
		widget.initResourceTableHeader()

		if widget.active {
			widget.startResourceWatch(widget.selectedRD)
		}
	case 2:
		// Drop back to resource list for the new node
		widget.level = 1
		widget.pages.SwitchToPage(pageResourceList)
		widget.resources = make(map[string]resource.Resource)
		widget.sortedIDs = nil
		widget.initResourceTableHeader()

		if widget.active {
			widget.app.SetFocus(widget.resourceTable)
			widget.startResourceWatch(widget.selectedRD)
		}
	}
}

// activateFilter shows the filter input below the types table.
func (widget *ResourceExplorerGrid) activateFilter() {
	if widget.filterActive {
		return
	}

	widget.filterActive = true
	widget.filterInput.SetText(widget.filterText)
	widget.typesWrapper.SetRows(0, 1)
	widget.typesWrapper.AddItem(widget.filterInput, 1, 0, 1, 1, 0, 0, true)
	widget.app.SetFocus(widget.filterInput)
}

// deactivateFilter hides the filter input. If clearText is true, the filter is also cleared.
func (widget *ResourceExplorerGrid) deactivateFilter(clearText bool) {
	if !widget.filterActive {
		if clearText && widget.filterText != "" {
			widget.filterText = ""
			widget.filterInput.SetText("")

			if widget.resourceDefs != nil {
				widget.renderTypesTable()
			}
		}

		return
	}

	if clearText {
		widget.filterText = ""
		widget.filterInput.SetText("")
	}

	widget.filterActive = false
	widget.typesWrapper.RemoveItem(widget.filterInput)
	widget.typesWrapper.SetRows(0)

	if clearText && widget.resourceDefs != nil {
		widget.renderTypesTable()
	}

	if widget.active {
		widget.app.SetFocus(widget.typesTable)
	}
}

// loadResourceTypes asynchronously fetches the list of resource definitions.
func (widget *ResourceExplorerGrid) loadResourceTypes() {
	widget.initTypesTableHeader()
	widget.typesTable.SetCell(1, 0, &tview.TableCell{
		Text:          "[gray]Loading...[-]",
		NotSelectable: true,
	})

	ctx := nodeContext(widget.ctx, widget.selectedNode)

	go func() {
		list, err := safe.StateListAll[*meta.ResourceDefinition](ctx, widget.dashboard.cli.COSI)
		if err != nil {
			widget.app.QueueUpdateDraw(func() {
				widget.initTypesTableHeader()
				widget.typesTable.SetCell(1, 0, &tview.TableCell{
					Text:          fmt.Sprintf("[red]%s[-]", formatError(err)),
					NotSelectable: true,
				})
			})

			return
		}

		defs := make([]*meta.ResourceDefinition, 0, list.Len())

		for rd := range list.All() {
			defs = append(defs, rd)
		}

		sort.Slice(defs, func(i, j int) bool {
			return defs[i].TypedSpec().Type < defs[j].TypedSpec().Type
		})

		widget.app.QueueUpdateDraw(func() {
			widget.resourceDefs = defs
			widget.renderTypesTable()
		})
	}()
}

// initTypesTableHeader sets the header row for the types table.
func (widget *ResourceExplorerGrid) initTypesTableHeader() {
	widget.typesTable.Clear()
	widget.typesTable.SetCell(0, 0, headerCell("TYPE"))
	widget.typesTable.SetCell(0, 1, headerCell("NAMESPACE"))
	widget.typesTable.SetCell(0, 2, headerCell("ALIASES"))
}

// renderTypesTable rebuilds the resource types table, applying the current filter.
func (widget *ResourceExplorerGrid) renderTypesTable() {
	widget.initTypesTableHeader()

	filter := strings.ToLower(widget.filterText)

	row := 1

	for _, rd := range widget.resourceDefs {
		spec := rd.TypedSpec()

		if filter != "" {
			matched := strings.Contains(strings.ToLower(spec.Type), filter)

			if !matched {
				for _, alias := range spec.AllAliases {
					if strings.Contains(strings.ToLower(alias), filter) {
						matched = true

						break
					}
				}
			}

			if !matched {
				continue
			}
		}

		widget.typesTable.SetCell(row, 0, &tview.TableCell{
			Text:      spec.Type,
			Align:     tview.AlignLeft,
			Color:     tcell.ColorWhite,
			Reference: rd, // used by selectResourceType to retrieve the RD
			Expansion: 1,
		})
		widget.typesTable.SetCell(row, 1, &tview.TableCell{
			Text:  spec.DefaultNamespace,
			Align: tview.AlignLeft,
			Color: tcell.ColorWhite,
		})
		widget.typesTable.SetCell(row, 2, &tview.TableCell{
			Text:  strings.Join(spec.Aliases, ", "),
			Align: tview.AlignLeft,
			Color: tcell.ColorGray,
		})

		row++
	}

	// Always start at the top of the list
	widget.typesTable.SetOffset(0, 0)

	if widget.typesTable.GetRowCount() > 1 {
		widget.typesTable.Select(1, 0)
	}
}

// selectResourceType is called when the user presses Enter on a resource type row.
func (widget *ResourceExplorerGrid) selectResourceType(row int) {
	if row == 0 {
		return
	}

	cell := widget.typesTable.GetCell(row, 0)
	if cell == nil || cell.Reference == nil {
		return
	}

	rd, ok := cell.Reference.(*meta.ResourceDefinition)
	if !ok {
		return
	}

	widget.selectedRD = rd

	spec := rd.TypedSpec()
	widget.resourceTable.SetTitle(fmt.Sprintf(" %s (ns: %s) — Enter: view YAML, Esc: back ", spec.DisplayType, spec.DefaultNamespace))

	widget.buildDynamicColumns(rd)

	widget.resources = make(map[string]resource.Resource)
	widget.sortedIDs = nil
	widget.initResourceTableHeader()

	widget.level = 1
	widget.pages.SwitchToPage(pageResourceList)
	widget.app.SetFocus(widget.resourceTable)

	widget.startResourceWatch(rd)
}

// buildDynamicColumns compiles the jsonpath expressions from the resource definition's PrintColumns.
func (widget *ResourceExplorerGrid) buildDynamicColumns(rd *meta.ResourceDefinition) {
	cols := rd.TypedSpec().PrintColumns

	widget.dynamicColumnNames = make([]string, 0, len(cols))
	widget.dynamicColumns = make([]func(any) (string, error), 0, len(cols))

	for _, col := range cols {
		expr := jsonpath.New(col.Name).AllowMissingKeys(true)
		if err := expr.Parse(col.JSONPath); err != nil {
			// skip columns whose jsonpath can't be compiled
			continue
		}

		widget.dynamicColumnNames = append(widget.dynamicColumnNames, strings.ToUpper(col.Name))

		capturedExpr := expr

		widget.dynamicColumns = append(widget.dynamicColumns, func(val any) (string, error) {
			var buf bytes.Buffer
			if e := capturedExpr.Execute(&buf, val); e != nil {
				return "", e
			}

			return buf.String(), nil
		})
	}
}

// initResourceTableHeader sets the header row for the resource instances table.
func (widget *ResourceExplorerGrid) initResourceTableHeader() {
	widget.resourceTable.Clear()
	widget.resourceTable.SetCell(0, 0, headerCell("ID"))
	widget.resourceTable.SetCell(0, 1, headerCell("VERSION"))
	widget.resourceTable.SetCell(0, 2, headerCell("PHASE"))
	widget.resourceTable.SetCell(0, 3, headerCell("OWNER"))

	for i, name := range widget.dynamicColumnNames {
		widget.resourceTable.SetCell(0, 4+i, headerCell(name))
	}

	widget.resourceTable.SetCell(1, 0, &tview.TableCell{
		Text:          "[gray]Loading...[-]",
		NotSelectable: true,
	})
}

// renderResourceTable rebuilds the resource instances table from widget.resources.
func (widget *ResourceExplorerGrid) renderResourceTable() {
	ids := make([]string, 0, len(widget.resources))
	for id := range widget.resources {
		ids = append(ids, id)
	}

	sort.Strings(ids)
	widget.sortedIDs = ids

	// Remember the previously selected row to preserve it after a live update
	selectedRow, _ := widget.resourceTable.GetSelection()

	widget.initResourceTableHeader()
	widget.resourceTable.RemoveRow(1) // remove "loading" header

	for i, id := range ids {
		res := widget.resources[id]
		md := res.Metadata()

		phaseText := tview.Escape(md.Phase().String())
		phaseColor := tcell.ColorWhite

		if md.Phase() == resource.PhaseTearingDown {
			phaseText = "[red]" + phaseText + "[-]"
			phaseColor = tcell.ColorDefault
		}

		widget.resourceTable.SetCell(i+1, 0, &tview.TableCell{
			Text:      tview.Escape(md.ID()),
			Align:     tview.AlignLeft,
			Color:     tcell.ColorWhite,
			Expansion: 1,
		})
		widget.resourceTable.SetCell(i+1, 1, &tview.TableCell{
			Text:  tview.Escape(md.Version().String()),
			Align: tview.AlignLeft,
			Color: tcell.ColorGray,
		})
		widget.resourceTable.SetCell(i+1, 2, &tview.TableCell{
			Text:  phaseText,
			Align: tview.AlignLeft,
			Color: phaseColor,
		})
		widget.resourceTable.SetCell(i+1, 3, &tview.TableCell{
			Text:  tview.Escape(md.Owner()),
			Align: tview.AlignLeft,
			Color: tcell.ColorGray,
		})

		if len(widget.dynamicColumns) > 0 {
			specVal := widget.marshalSpec(res)

			for j, dynCol := range widget.dynamicColumns {
				text, err := dynCol(specVal)
				if err != nil {
					text = ""
				}

				widget.resourceTable.SetCell(i+1, 4+j, &tview.TableCell{
					Text:  tview.Escape(text),
					Align: tview.AlignLeft,
					Color: tcell.ColorWhite,
				})
			}
		}
	}

	// Restore selection if still valid, otherwise go to first row
	rowCount := widget.resourceTable.GetRowCount()
	if selectedRow > 0 && selectedRow < rowCount {
		widget.resourceTable.Select(selectedRow, 0)
	} else if rowCount > 1 {
		// Initial render: reset scroll to top before selecting the first row.
		widget.resourceTable.SetOffset(0, 0)
		widget.resourceTable.Select(1, 0)
	}
}

// startResourceWatch begins watching resources of the given kind.
func (widget *ResourceExplorerGrid) startResourceWatch(rd *meta.ResourceDefinition) {
	widget.stopResourceWatch()

	ctx, cancel := context.WithCancel(widget.ctx)
	widget.watchCancel = cancel

	go widget.runResourceWatch(ctx, rd)
}

// stopResourceWatch cancels a running resource watch if any.
func (widget *ResourceExplorerGrid) stopResourceWatch() {
	if widget.watchCancel != nil {
		widget.watchCancel()
		widget.watchCancel = nil
	}
}

// runResourceWatch runs the WatchKind event loop for the given resource definition.
//
//nolint:gocyclo
func (widget *ResourceExplorerGrid) runResourceWatch(ctx context.Context, rd *meta.ResourceDefinition) {
	nodeCtx := nodeContext(ctx, widget.selectedNode)

	spec := rd.TypedSpec()

	eventCh := make(chan state.Event)

	md := resource.NewMetadata(spec.DefaultNamespace, spec.Type, "", resource.VersionUndefined)

	if err := widget.dashboard.cli.COSI.WatchKind(
		nodeCtx, &md, eventCh,
		state.WithBootstrapContents(true),
		state.WithWatchKindUnmarshalOptions(state.WithSkipProtobufUnmarshal()),
	); err != nil {
		widget.app.QueueUpdateDraw(func() {
			if widget.selectedRD == rd {
				widget.showResourceTableError(formatError(err))
			}
		})

		return
	}

	bootstrapped := false

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-eventCh:
			switch event.Type {
			case state.Errored:
				widget.app.QueueUpdateDraw(func() {
					if widget.selectedRD == rd {
						widget.showResourceTableError(formatError(event.Error))
					}
				})

				return
			case state.Bootstrapped:
				bootstrapped = true

				widget.app.QueueUpdateDraw(func() {
					if widget.selectedRD == rd && widget.level >= 1 {
						widget.renderResourceTable()
					}
				})
			case state.Noop:
				// nothing to do
			case state.Created, state.Updated:
				res := event.Resource
				wasBootstrapped := bootstrapped

				widget.app.QueueUpdateDraw(func() {
					if widget.selectedRD == rd && widget.level >= 1 {
						widget.resources[res.Metadata().ID()] = res

						if wasBootstrapped {
							widget.renderResourceTable()
						}
					}
				})
			case state.Destroyed:
				res := event.Resource
				wasBootstrapped := bootstrapped

				widget.app.QueueUpdateDraw(func() {
					if widget.selectedRD == rd && widget.level >= 1 {
						delete(widget.resources, res.Metadata().ID())

						if wasBootstrapped {
							widget.renderResourceTable()
						}
					}
				})
			}
		}
	}
}

// marshalSpec marshals the resource spec to an unstructured value suitable for jsonpath evaluation.
func (widget *ResourceExplorerGrid) marshalSpec(res resource.Resource) any {
	out, err := yaml.Marshal(res.Spec())
	if err != nil {
		return nil
	}

	var unstructured any

	if err = yaml.Unmarshal(out, &unstructured); err != nil {
		return nil
	}

	return unstructured
}

// showResourceTableError displays an error message in the resource instances table.
func (widget *ResourceExplorerGrid) showResourceTableError(msg string) {
	widget.initResourceTableHeader()
	widget.resourceTable.SetCell(1, 0, &tview.TableCell{
		Text:          fmt.Sprintf("[red]%s[-]", tview.Escape(msg)),
		NotSelectable: true,
		Expansion:     1,
	})
}

// selectResource is called when the user presses Enter on a resource instance row.
func (widget *ResourceExplorerGrid) selectResource(row int) {
	if row == 0 || row-1 >= len(widget.sortedIDs) {
		return
	}

	id := widget.sortedIDs[row-1]

	res, ok := widget.resources[id]
	if !ok {
		return
	}

	widget.showResourceYAML(res)
}

// showResourceYAML renders the YAML of the given resource and shows the YAML view.
func (widget *ResourceExplorerGrid) showResourceYAML(res resource.Resource) {
	out, err := resource.MarshalYAML(res)
	if err != nil {
		widget.yamlView.SetText(fmt.Sprintf("Error marshaling resource: %v", err))
	} else {
		outBytes, marshalErr := yaml.Marshal(out)
		if marshalErr != nil {
			widget.yamlView.SetText(fmt.Sprintf("Error encoding YAML: %v", marshalErr))
		} else {
			var node yaml.Node

			if unmarshalErr := yaml.Unmarshal(outBytes, &node); unmarshalErr != nil {
				widget.yamlView.SetText(fmt.Sprintf("Error encoding YAML: %v", unmarshalErr))
			} else {
				var sb strings.Builder

				renderYAMLNode(&sb, &node, 0, false)
				widget.yamlView.SetText(sb.String())
			}
		}
	}

	widget.yamlView.SetTitle(fmt.Sprintf(" %s / %s (Esc: back) ", res.Metadata().Namespace(), res.Metadata().ID()))
	widget.yamlView.ScrollToBeginning()
	widget.level = 2
	widget.pages.SwitchToPage(pageResourceYAML)
	widget.app.SetFocus(widget.yamlView)
}

// goBack returns to the previous navigation level.
func (widget *ResourceExplorerGrid) goBack() {
	switch widget.level {
	case 2:
		widget.level = 1
		widget.pages.SwitchToPage(pageResourceList)
		widget.app.SetFocus(widget.resourceTable)
	case 1:
		widget.stopResourceWatch()
		widget.resources = make(map[string]resource.Resource)
		widget.sortedIDs = nil
		widget.level = 0
		widget.pages.SwitchToPage(pageResourceTypes)
		widget.app.SetFocus(widget.typesTable)
	}
}

// formatError returns a user-friendly error message.
// Permission denied errors are given a clear explanation since sensitive resources
// are only accessible with elevated privileges.
func formatError(err error) string {
	if client.StatusCode(err) == codes.PermissionDenied {
		return "access denied: this resource is sensitive and requires elevated permissions"
	}

	return err.Error()
}

// renderYAMLNode walks a yaml.Node AST and writes a tview-colored representation
// into sb. indent is the current indentation level; inSequence signals that the
// caller is rendering a sequence item and has already written the "- " prefix.
//
// Color scheme:
//
//	cyan   – mapping keys
//	green  – string scalars
//	yellow – numeric, boolean, and null scalars
//
//nolint:gocyclo
func renderYAMLNode(sb *strings.Builder, node *yaml.Node, indent int, inSequence bool) {
	prefix := strings.Repeat("  ", indent)
	seqPrefix := strings.Repeat("  ", indent) + "- "

	switch node.Kind { //nolint:exhaustive
	case yaml.DocumentNode:
		for _, child := range node.Content {
			renderYAMLNode(sb, child, indent, false)
		}

	case yaml.MappingNode:
		// Content is key/value pairs interleaved: [k0, v0, k1, v1, …]
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			val := node.Content[i+1]

			linePrefix := prefix
			if inSequence && i == 0 {
				linePrefix = seqPrefix
			}

			sb.WriteString(linePrefix)
			fmt.Fprintf(sb, "[darkcyan]%s[-]:", tview.Escape(key.Value))

			switch val.Kind { //nolint:exhaustive
			case yaml.ScalarNode:
				sb.WriteString(" ")
				writeScalar(sb, val)
				sb.WriteString("\n")
			case yaml.MappingNode, yaml.SequenceNode:
				sb.WriteString("\n")
				renderYAMLNode(sb, val, indent+1, false)
			default:
				sb.WriteString("\n")
				renderYAMLNode(sb, val, indent+1, false)
			}

			inSequence = false // only the first key gets the "- " prefix
		}

	case yaml.SequenceNode:
		for _, item := range node.Content {
			switch item.Kind { //nolint:exhaustive
			case yaml.ScalarNode:
				sb.WriteString(seqPrefix)
				writeScalar(sb, item)
				sb.WriteString("\n")
			case yaml.MappingNode:
				renderYAMLNode(sb, item, indent, true)
			default:
				sb.WriteString(seqPrefix + "\n")
				renderYAMLNode(sb, item, indent+1, false)
			}
		}

	case yaml.ScalarNode:
		sb.WriteString(prefix)
		writeScalar(sb, node)
		sb.WriteString("\n")

	case yaml.AliasNode:
		renderYAMLNode(sb, node.Alias, indent, inSequence)
	}
}

// writeScalar writes a single colored scalar value into sb.
func writeScalar(sb *strings.Builder, node *yaml.Node) {
	switch node.ShortTag() {
	case "!!str":
		// Preserve the original quoting style in the display.
		var quoted string

		switch node.Style { //nolint:exhaustive
		case yaml.DoubleQuotedStyle:
			quoted = `"` + tview.Escape(node.Value) + `"`
		case yaml.SingleQuotedStyle:
			quoted = `'` + tview.Escape(node.Value) + `'`
		case yaml.LiteralStyle:
			quoted = "|\n" + tview.Escape(node.Value)
		case yaml.FoldedStyle:
			quoted = ">\n" + tview.Escape(node.Value)
		default:
			quoted = tview.Escape(node.Value)
		}

		fmt.Fprintf(sb, "[green]%s[-]", quoted)

	case "!!int", "!!float":
		fmt.Fprintf(sb, "[yellow]%s[-]", tview.Escape(node.Value))

	case "!!bool":
		fmt.Fprintf(sb, "[yellow]%s[-]", tview.Escape(node.Value))

	case "!!null":
		fmt.Fprintf(sb, "[yellow]%s[-]", tview.Escape(node.Value))

	default:
		sb.WriteString(tview.Escape(node.Value))
	}
}

// headerCell creates a bold header cell for tables.
func headerCell(text string) *tview.TableCell {
	return &tview.TableCell{
		Text:          "[::b]" + text,
		Align:         tview.AlignLeft,
		NotSelectable: true,
		Color:         tcell.ColorWhite,
	}
}
