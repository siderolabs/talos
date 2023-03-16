// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/siderolabs/gen/maps"

	"github.com/siderolabs/talos/internal/pkg/dashboard/resourcedata"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type networkConfigData struct {
	linkSet map[string]struct{}
}

// NetworkConfigGrid represents the network configuration widget.
type NetworkConfigGrid struct {
	tview.Grid

	app *tview.Application

	configForm        *tview.Form
	hostnameField     *tview.InputField
	dnsServersField   *tview.InputField
	timeServersField  *tview.InputField
	interfaceDropdown *tview.DropDown
	modeDropdown      *tview.DropDown
	addressesField    *tview.InputField
	gatewayField      *tview.InputField

	infoView *tview.TextView

	selectedNode string
	nodeMap      map[string]*networkConfigData
}

// NewNetworkConfigGrid initializes NetworkConfigGrid.
func NewNetworkConfigGrid(app *tview.Application) *NetworkConfigGrid {
	widget := &NetworkConfigGrid{
		Grid:       *tview.NewGrid(),
		app:        app,
		configForm: tview.NewForm(),
		infoView:   tview.NewTextView(),
		nodeMap:    make(map[string]*networkConfigData),
	}

	widget.infoView.SetBorderPadding(1, 0, 1, 0)

	widget.configForm.SetBorder(true)

	widget.SetRows(0).SetColumns(0, 0, 0)

	widget.AddItem(tview.NewBox(), 0, 0, 1, 1, 0, 0, false)
	widget.AddItem(widget.configForm, 0, 1, 1, 1, 0, 0, false)
	widget.AddItem(widget.infoView, 0, 2, 1, 1, 0, 0, false)

	widget.hostnameField = tview.NewInputField().SetLabel("Hostname")
	widget.dnsServersField = tview.NewInputField().SetLabel("DNS Servers")
	widget.timeServersField = tview.NewInputField().SetLabel("Time Servers")
	widget.interfaceDropdown = tview.NewDropDown().SetLabel("Interface")
	widget.modeDropdown = tview.NewDropDown().SetLabel("Mode")
	widget.addressesField = tview.NewInputField().SetLabel("Addresses")
	widget.gatewayField = tview.NewInputField().SetLabel("Gateway")

	widget.configForm.AddFormItem(widget.hostnameField)
	widget.configForm.AddFormItem(widget.dnsServersField)
	widget.configForm.AddFormItem(widget.timeServersField)
	widget.configForm.AddFormItem(widget.interfaceDropdown)
	widget.configForm.AddFormItem(widget.modeDropdown)
	widget.configForm.AddFormItem(widget.addressesField)
	widget.configForm.AddFormItem(widget.gatewayField)
	widget.configForm.AddButton("Save", func() {
		widget.save()
	})

	widget.interfaceDropdown.SetSelectedFunc(func(text string, index int) {
		// TODO(dashboard): Clear the form & load existing config for the selected interface.
	})

	widget.modeDropdown.SetOptions([]string{"No Config", "DHCP", "Static"}, func(text string, _ int) {
		switch text {
		case "Static":
			if itemIndex := widget.configForm.GetFormItemIndex("Addresses"); itemIndex == -1 {
				widget.configForm.AddFormItem(widget.addressesField)
			}

			if itemIndex := widget.configForm.GetFormItemIndex("Gateway"); itemIndex == -1 {
				widget.configForm.AddFormItem(widget.gatewayField)
			}
		default:
			if itemIndex := widget.configForm.GetFormItemIndex("Addresses"); itemIndex != -1 {
				widget.configForm.RemoveFormItem(itemIndex)
			}

			if itemIndex := widget.configForm.GetFormItemIndex("Gateway"); itemIndex != -1 {
				widget.configForm.RemoveFormItem(itemIndex)
			}
		}
	})

	widget.configForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		formItemIndex, buttonIndex := widget.configForm.GetFocusedItemIndex()

		currIndex := formItemIndex
		if formItemIndex == -1 {
			currIndex = widget.configForm.GetFormItemCount() + buttonIndex
		}

		//nolint:exhaustive
		switch event.Key() {
		case tcell.KeyUp:
			widget.configForm.SetFocus(currIndex - 1)

			widget.app.SetFocus(widget.configForm)

			return nil
		case tcell.KeyDown:
			// prevent jumping to the first field if we are at the end of the form
			if currIndex < widget.configForm.GetFormItemCount()+widget.configForm.GetButtonCount()-1 {
				widget.configForm.SetFocus(currIndex + 1)
			}

			widget.app.SetFocus(widget.configForm)

			return nil
		default:
			return event
		}
	})

	widget.AddItem(widget.configForm, 0, 1, 1, 1, 0, 0, false)

	return widget
}

// OnNodeSelect implements the NodeSelectListener interface.
func (widget *NetworkConfigGrid) OnNodeSelect(node string) {
	if node != widget.selectedNode {
		widget.selectedNode = node

		widget.redraw()
	}
}

// OnResourceDataChange implements the ResourceDataListener interface.
func (widget *NetworkConfigGrid) OnResourceDataChange(data resourcedata.Data) {
	widget.updateNodeData(data)

	if data.Node == widget.selectedNode {
		widget.redraw()
	}
}

func (widget *NetworkConfigGrid) redraw() {
	// todo
}

func (widget *NetworkConfigGrid) updateNodeData(data resourcedata.Data) {
	nodeData := widget.getOrCreateNodeData(data.Node)

	//nolint:gocritic
	switch res := data.Resource.(type) {
	case *network.LinkStatus:
		if data.Deleted {
			delete(nodeData.linkSet, res.Metadata().ID())
		} else {
			if !res.TypedSpec().LinkState ||
				res.TypedSpec().Type == nethelpers.LinkLoopbck ||
				res.TypedSpec().Kind != "" {
				return
			}

			nodeData.linkSet[res.Metadata().ID()] = struct{}{}
		}

		links := maps.Keys(nodeData.linkSet)

		sort.Strings(links)

		widget.interfaceDropdown.SetOptions(links, nil)
	}
}

func (widget *NetworkConfigGrid) getOrCreateNodeData(node string) *networkConfigData {
	nodeData, ok := widget.nodeMap[node]
	if !ok {
		nodeData = &networkConfigData{
			linkSet: make(map[string]struct{}),
		}

		widget.nodeMap[node] = nodeData
	}

	return nodeData
}

// OnScreenSelect implements the screenSelectListener interface.
func (widget *NetworkConfigGrid) onScreenSelect(active bool) {
	if active {
		widget.app.SetFocus(widget.configForm)
	}
}

func (widget *NetworkConfigGrid) save() {
	_, iface := widget.interfaceDropdown.GetCurrentOption()
	_, mode := widget.modeDropdown.GetCurrentOption()

	formData := networkConfigFormData{
		hostname:    widget.hostnameField.GetText(),
		dnsServers:  widget.dnsServersField.GetText(),
		timeServers: widget.timeServersField.GetText(),
		iface:       iface,
		mode:        mode,
		addresses:   widget.addressesField.GetText(),
		gateway:     widget.gatewayField.GetText(),
	}

	platformNetworkConfig, err := formData.toPlatformNetworkConfig()
	if err != nil { // TODO(dashboard): show error
		return
	}

	_ = platformNetworkConfig // TODO(dashboard): save config
}
