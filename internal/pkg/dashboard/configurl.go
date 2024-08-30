// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"context"
	"fmt"
	"strings"

	"github.com/rivo/tview"
	"github.com/siderolabs/go-procfs/procfs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/pkg/dashboard/resourcedata"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const unset = "[gray](unset)[-]"

type configURLData struct {
	existingCode string
}

// ConfigURLGrid represents the config URL grid.
type ConfigURLGrid struct {
	tview.Grid

	dashboard *Dashboard

	form *tview.Form

	template     *tview.TextView
	existingCode *tview.TextView
	newCodeField *tview.InputField
	infoView     *tview.TextView

	selectedNode string
	nodeMap      map[string]*configURLData
}

// NewConfigURLGrid returns a new config URL grid.
func NewConfigURLGrid(ctx context.Context, dashboard *Dashboard) *ConfigURLGrid {
	grid := &ConfigURLGrid{
		Grid:         *tview.NewGrid(),
		dashboard:    dashboard,
		form:         tview.NewForm(),
		template:     tview.NewTextView().SetDynamicColors(true).SetLabel("Template").SetScrollable(false),
		existingCode: tview.NewTextView().SetDynamicColors(true).SetLabel("Existing Code").SetText(unset).SetSize(1, 0).SetScrollable(false),
		newCodeField: tview.NewInputField().SetLabel("New Code"),
		infoView:     tview.NewTextView().SetDynamicColors(true).SetSize(2, 0).SetScrollable(false),

		nodeMap: make(map[string]*configURLData),
	}

	grid.template.SetText(grid.readTemplateFromKernelArgs())

	grid.SetRows(-1, 18, -1).SetColumns(-1, 72, -1)

	grid.form.SetBorder(true)

	grid.form.AddFormItem(grid.template)
	grid.form.AddFormItem(grid.existingCode)
	grid.form.AddFormItem(grid.newCodeField)
	grid.form.AddButton("Save", func() {
		ctx = nodeContext(ctx, grid.selectedNode)

		value := grid.newCodeField.GetText()

		if value == "" {
			grid.infoView.SetText("[red]Error: No code entered[-]")

			return
		}

		err := dashboard.cli.MetaWrite(ctx, meta.DownloadURLCode, []byte(value))
		if err != nil {
			grid.infoView.SetText(fmt.Sprintf("[red]Error: %v[-]", err))

			return
		}

		grid.clearForm()
		grid.dashboard.selectScreen(ScreenSummary)
	})
	grid.form.AddButton("Delete", func() {
		ctx = nodeContext(ctx, grid.selectedNode)

		err := dashboard.cli.MetaDelete(ctx, meta.DownloadURLCode)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				grid.clearForm()
				grid.infoView.SetText("[green]Already deleted[-]")

				return
			}

			grid.infoView.SetText(fmt.Sprintf("[red]Error: %v[-]", err))

			return
		}

		grid.clearForm()
		grid.infoView.SetText("[green]Deleted successfully[-]")
	})

	grid.form.AddFormItem(grid.infoView)

	grid.AddItem(tview.NewBox(), 0, 0, 1, 3, 0, 0, false)
	grid.AddItem(tview.NewBox(), 1, 0, 1, 1, 0, 0, false)
	grid.AddItem(grid.form, 1, 1, 1, 1, 0, 0, false)
	grid.AddItem(tview.NewBox(), 1, 2, 1, 1, 0, 0, false)
	grid.AddItem(tview.NewBox(), 2, 0, 1, 3, 0, 0, false)

	return grid
}

func (widget *ConfigURLGrid) readTemplateFromKernelArgs() (val string) {
	defer func() { // catch potential panic from procfs.ProcCmdline()
		if r := recover(); r != nil {
			val = "error reading kernel args"
		}
	}()

	option := procfs.ProcCmdline().Get(constants.KernelParamConfig).First()
	if option == nil {
		return unset
	}

	codeVar := fmt.Sprintf("${%s}", constants.CodeKey)

	return strings.ReplaceAll(tview.Escape(*option), codeVar, fmt.Sprintf("[green]%s[-]", codeVar))
}

// OnScreenSelect implements the screenSelectListener interface.
func (widget *ConfigURLGrid) onScreenSelect(active bool) {
	if active {
		widget.dashboard.app.SetFocus(widget.form)
	} else {
		widget.clearForm()
	}
}

// OnNodeSelect implements the NodeSelectListener interface.
func (widget *ConfigURLGrid) OnNodeSelect(node string) {
	if node != widget.selectedNode {
		widget.selectedNode = node

		widget.clearForm()
		widget.redraw()
	}
}

// OnResourceDataChange implements the ResourceDataListener interface.
func (widget *ConfigURLGrid) OnResourceDataChange(data resourcedata.Data) {
	widget.updateNodeData(data)

	if data.Node == widget.selectedNode {
		widget.redraw()
	}
}

func (widget *ConfigURLGrid) updateNodeData(data resourcedata.Data) {
	nodeData := widget.getOrCreateNodeData(data.Node)

	//nolint:gocritic
	switch res := data.Resource.(type) {
	case *runtimeres.MetaKey:
		if res.Metadata().ID() == runtimeres.MetaKeyTagToID(meta.DownloadURLCode) {
			if data.Deleted {
				nodeData.existingCode = unset
			} else {
				val := res.TypedSpec().Value
				if val == "" {
					val = "(empty)"
				}

				nodeData.existingCode = fmt.Sprintf("[blue]%s[-]", val)
			}
		}
	}
}

func (widget *ConfigURLGrid) redraw() {
	data := widget.getOrCreateNodeData(widget.selectedNode)

	widget.existingCode.SetText(data.existingCode)
}

func (widget *ConfigURLGrid) getOrCreateNodeData(node string) *configURLData {
	nodeData, ok := widget.nodeMap[node]
	if !ok {
		nodeData = &configURLData{
			existingCode: unset,
		}

		widget.nodeMap[node] = nodeData
	}

	return nodeData
}

func (widget *ConfigURLGrid) clearForm() {
	widget.form.SetFocus(0)
	widget.infoView.SetText("")
	widget.newCodeField.SetText("")
}
