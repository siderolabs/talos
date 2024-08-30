// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/go-pointer"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/dashboard/resourcedata"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const (
	formItemHostname    = "Hostname"
	formItemDNSServers  = "DNS Servers"
	formItemTimeServers = "Time Servers"
	formItemInterface   = "Interface"
	formItemMode        = "Mode"
	formItemAddresses   = "Addresses"
	formItemGateway     = "Gateway"
)

type networkConfigData struct {
	existingConfig *runtime.PlatformNetworkConfig
	newConfig      *runtime.PlatformNetworkConfig
	newConfigError error
	linkSet        map[string]struct{}
}

// NetworkConfigGrid represents the network configuration widget.
type NetworkConfigGrid struct {
	tview.Grid

	dashboard *Dashboard

	configForm        *tview.Form
	hostnameField     *tview.InputField
	dnsServersField   *tview.InputField
	timeServersField  *tview.InputField
	interfaceDropdown *tview.DropDown
	modeDropdown      *tview.DropDown
	addressesField    *tview.InputField
	gatewayField      *tview.InputField

	infoView           *tview.TextView
	existingConfigView *tview.TextView
	newConfigView      *tview.TextView

	selectedNode string
	nodeMap      map[string]*networkConfigData
}

// NewNetworkConfigGrid initializes NetworkConfigGrid.
func NewNetworkConfigGrid(ctx context.Context, dashboard *Dashboard) *NetworkConfigGrid {
	widget := &NetworkConfigGrid{
		Grid:               *tview.NewGrid(),
		configForm:         tview.NewForm(),
		infoView:           tview.NewTextView(),
		existingConfigView: tview.NewTextView(),
		newConfigView:      tview.NewTextView(),
		nodeMap:            make(map[string]*networkConfigData),
		dashboard:          dashboard,
	}

	widget.configForm.SetBorder(true).SetTitle("Configure (Ctrl+Q)")
	widget.SetRows(0, 3).SetColumns(0, 0, 0)

	widget.infoView.
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	widget.existingConfigView.
		SetDynamicColors(true).
		SetScrollable(true).
		SetBorderPadding(0, 0, 1, 0).
		SetBorder(true).
		SetTitle("Existing Config (Ctrl+W)")
	widget.newConfigView.
		SetDynamicColors(true).
		SetScrollable(true).
		SetBorderPadding(0, 0, 1, 0).
		SetBorder(true).
		SetTitle("New Config (Ctrl+E)")

	widget.AddItem(widget.configForm, 0, 0, 1, 1, 0, 0, false)
	widget.AddItem(widget.infoView, 1, 0, 1, 1, 0, 0, false)
	widget.AddItem(widget.existingConfigView, 0, 1, 2, 1, 0, 0, false)
	widget.AddItem(widget.newConfigView, 0, 2, 2, 1, 0, 0, false)

	widget.hostnameField = tview.NewInputField().SetLabel(formItemHostname)
	widget.hostnameField.SetBlurFunc(widget.formEdited)

	widget.dnsServersField = tview.NewInputField().SetLabel(formItemDNSServers)
	widget.dnsServersField.SetBlurFunc(widget.formEdited)

	widget.timeServersField = tview.NewInputField().SetLabel(formItemTimeServers)
	widget.timeServersField.SetBlurFunc(widget.formEdited)

	widget.interfaceDropdown = tview.NewDropDown().SetLabel(formItemInterface)
	widget.interfaceDropdown.SetBlurFunc(widget.formEdited)
	widget.interfaceDropdown.SetOptions([]string{interfaceNone}, func(_ string, _ int) {
		widget.formEdited()
	})
	widget.interfaceDropdown.SetListStyles(
		tcell.StyleDefault.Foreground(tview.Styles.PrimitiveBackgroundColor).Background(tview.Styles.MoreContrastBackgroundColor),
		tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tview.Styles.PrimaryTextColor),
	)

	widget.modeDropdown = tview.NewDropDown().SetLabel(formItemMode)
	widget.modeDropdown.SetBlurFunc(widget.formEdited)
	widget.modeDropdown.SetOptions([]string{ModeDHCP, ModeStatic}, func(_ string, _ int) {
		widget.formEdited()
	})
	widget.modeDropdown.SetListStyles(
		tcell.StyleDefault.Foreground(tview.Styles.PrimitiveBackgroundColor).Background(tview.Styles.MoreContrastBackgroundColor),
		tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tview.Styles.PrimaryTextColor),
	)

	widget.addressesField = tview.NewInputField().SetLabel(formItemAddresses)
	widget.addressesField.SetBlurFunc(widget.formEdited)

	widget.gatewayField = tview.NewInputField().SetLabel(formItemGateway)
	widget.gatewayField.SetBlurFunc(widget.formEdited)

	widget.configForm.AddFormItem(widget.hostnameField)
	widget.configForm.AddFormItem(widget.dnsServersField)
	widget.configForm.AddFormItem(widget.timeServersField)
	widget.configForm.AddFormItem(widget.interfaceDropdown)
	widget.configForm.AddFormItem(widget.modeDropdown)
	widget.configForm.AddFormItem(widget.addressesField)
	widget.configForm.AddFormItem(widget.gatewayField)

	widget.configForm.AddButton("Save", func() {
		widget.save(ctx)
	})

	saveButton := widget.configForm.GetButton(0)
	saveButton.SetBlurFunc(widget.formEdited)

	inputCapture := func(event *tcell.EventKey) *tcell.EventKey {
		if widget.handleFocusSwitch(event) {
			return nil
		}

		return event
	}

	widget.configForm.SetInputCapture(inputCapture)
	widget.existingConfigView.SetInputCapture(inputCapture)
	widget.newConfigView.SetInputCapture(inputCapture)

	widget.interfaceDropdown.SetCurrentOption(0)
	widget.modeDropdown.SetCurrentOption(0)

	return widget
}

// OnNodeSelect implements the NodeSelectListener interface.
func (widget *NetworkConfigGrid) OnNodeSelect(node string) {
	if node != widget.selectedNode {
		widget.selectedNode = node

		widget.clearForm()
		widget.formEdited()

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

//nolint:gocyclo
func (widget *NetworkConfigGrid) formEdited() {
	widget.infoView.SetText("")

	resetInputField := func(field *tview.InputField) {
		// avoid triggering another form edit if there is nothing to change
		if field.GetText() != "" {
			field.SetText("")
		}
	}

	resetDropdown := func(dropdown *tview.DropDown) {
		// avoid triggering another form edit if there is nothing to change
		if currentIndex, _ := dropdown.GetCurrentOption(); currentIndex != 0 {
			dropdown.SetCurrentOption(0)
		}
	}

	_, currentInterface := widget.interfaceDropdown.GetCurrentOption()
	_, currentMode := widget.modeDropdown.GetCurrentOption()

	ifaceSelected := currentInterface != "" && currentInterface != interfaceNone
	if ifaceSelected {
		if itemIndex := widget.configForm.GetFormItemIndex(formItemMode); itemIndex == -1 {
			widget.configForm.AddFormItem(widget.modeDropdown)
		}

		switch currentMode {
		case ModeDHCP:
			resetInputField(widget.addressesField)
			resetInputField(widget.gatewayField)

			if itemIndex := widget.configForm.GetFormItemIndex(formItemAddresses); itemIndex != -1 {
				widget.configForm.RemoveFormItem(itemIndex)
			}

			if itemIndex := widget.configForm.GetFormItemIndex(formItemGateway); itemIndex != -1 {
				widget.configForm.RemoveFormItem(itemIndex)
			}
		case ModeStatic:
			if itemIndex := widget.configForm.GetFormItemIndex(formItemAddresses); itemIndex == -1 {
				widget.configForm.AddFormItem(widget.addressesField)
			}

			if itemIndex := widget.configForm.GetFormItemIndex(formItemGateway); itemIndex == -1 {
				widget.configForm.AddFormItem(widget.gatewayField)
			}
		}
	} else {
		resetDropdown(widget.modeDropdown)
		resetInputField(widget.addressesField)
		resetInputField(widget.gatewayField)

		if itemIndex := widget.configForm.GetFormItemIndex(formItemMode); itemIndex != -1 {
			widget.configForm.RemoveFormItem(itemIndex)
		}

		if itemIndex := widget.configForm.GetFormItemIndex(formItemAddresses); itemIndex != -1 {
			widget.configForm.RemoveFormItem(itemIndex)
		}

		if itemIndex := widget.configForm.GetFormItemIndex(formItemGateway); itemIndex != -1 {
			widget.configForm.RemoveFormItem(itemIndex)
		}
	}

	data := widget.getOrCreateNodeData(widget.selectedNode)

	formData := NetworkConfigFormData{
		Base:        pointer.SafeDeref(data.existingConfig),
		Hostname:    widget.hostnameField.GetText(),
		DNSServers:  widget.dnsServersField.GetText(),
		TimeServers: widget.timeServersField.GetText(),
		Iface:       currentInterface,
		Mode:        currentMode,
		Addresses:   widget.addressesField.GetText(),
		Gateway:     widget.gatewayField.GetText(),
	}

	config, err := formData.ToPlatformNetworkConfig()
	if err != nil {
		data.newConfig = nil
		data.newConfigError = err
	} else {
		data.newConfig = config
		data.newConfigError = nil
	}

	widget.redraw()
}

func (widget *NetworkConfigGrid) redraw() {
	data := widget.getOrCreateNodeData(widget.selectedNode)

	if data.existingConfig != nil {
		var buf strings.Builder

		encoder := yaml.NewEncoder(&buf)
		encoder.SetIndent(2)

		err := encoder.Encode(data.existingConfig)
		if err != nil {
			widget.existingConfigView.SetText(fmt.Sprintf("[red]error: %v[-]", err))
		}

		widget.existingConfigView.SetText(fmt.Sprintf("[lightblue]%s[-]", tview.Escape(buf.String())))
	} else {
		widget.existingConfigView.SetText("[gray]No Config[-]")
	}

	if data.newConfigError != nil {
		widget.newConfigView.SetText(fmt.Sprintf("[red]error: %v[-]", data.newConfigError))
	} else if data.newConfig != nil {
		var buf strings.Builder

		encoder := yaml.NewEncoder(&buf)
		encoder.SetIndent(2)

		err := encoder.Encode(data.newConfig)
		if err != nil {
			widget.newConfigView.SetText(fmt.Sprintf("[red]error: %v[-]", err))
		}

		widget.newConfigView.SetText(fmt.Sprintf("[green]%s[-]", tview.Escape(buf.String())))
	}
}

func (widget *NetworkConfigGrid) clearForm() {
	widget.hostnameField.SetText("")
	widget.dnsServersField.SetText("")
	widget.timeServersField.SetText("")
	widget.interfaceDropdown.SetCurrentOption(0)
	widget.modeDropdown.SetCurrentOption(0)
	widget.addressesField.SetText("")
	widget.gatewayField.SetText("")
	widget.infoView.SetText("")

	widget.configForm.SetFocus(0)

	widget.formEdited()
}

func (widget *NetworkConfigGrid) updateNodeData(data resourcedata.Data) {
	nodeData := widget.getOrCreateNodeData(data.Node)

	switch res := data.Resource.(type) {
	case *network.LinkStatus:
		if data.Deleted {
			delete(nodeData.linkSet, res.Metadata().ID())
		} else {
			if !res.TypedSpec().Physical() {
				return
			}

			nodeData.linkSet[res.Metadata().ID()] = struct{}{}
		}

		links := maps.Keys(nodeData.linkSet)

		sort.Strings(links)

		allLinks := append([]string{interfaceNone}, links...)

		widget.interfaceDropdown.SetOptions(allLinks, func(_ string, _ int) {
			widget.formEdited()
		})
	case *runtimeres.MetaKey:
		if res.Metadata().ID() == runtimeres.MetaKeyTagToID(meta.MetalNetworkPlatformConfig) {
			if data.Deleted {
				nodeData.existingConfig = nil
			} else {
				cfg := runtime.PlatformNetworkConfig{}

				if err := yaml.Unmarshal([]byte(res.TypedSpec().Value), &cfg); err != nil {
					widget.existingConfigView.SetText(fmt.Sprintf("[red]error: %v[-]", err))

					return
				}

				nodeData.existingConfig = &cfg

				widget.formEdited()
			}
		}
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
		widget.dashboard.app.SetFocus(widget.configForm)
	}
}

func (widget *NetworkConfigGrid) save(ctx context.Context) {
	nodeData := widget.getOrCreateNodeData(widget.selectedNode)

	if nodeData.newConfig == nil {
		widget.infoView.SetText("[red]Error: nothing to save[-]")

		return
	}

	if nodeData.newConfigError != nil {
		widget.infoView.SetText("[red]Error: cannot save, fix the errors and try again[-]")

		return
	}

	configBytes, err := yaml.Marshal(nodeData.newConfig)
	if err != nil {
		widget.infoView.SetText(fmt.Sprintf("[red]Error: %v[-]", err))

		return
	}

	ctx = nodeContext(ctx, widget.selectedNode)

	if err = widget.dashboard.cli.MetaWrite(ctx, meta.MetalNetworkPlatformConfig, configBytes); err != nil {
		widget.infoView.SetText(fmt.Sprintf("[red]Error: %v[-]", err))

		return
	}

	widget.infoView.SetText("[green]Network config saved successfully[-]")
	widget.clearForm()
	widget.dashboard.selectScreen(ScreenSummary)
}

func (widget *NetworkConfigGrid) handleFocusSwitch(event *tcell.EventKey) bool {
	switch event.Key() { //nolint:exhaustive
	case tcell.KeyCtrlQ:
		widget.dashboard.app.SetFocus(widget.configForm)

		return true
	case tcell.KeyCtrlW:
		widget.dashboard.app.SetFocus(widget.existingConfigView)

		return true
	case tcell.KeyCtrlE:
		widget.dashboard.app.SetFocus(widget.newConfigView)

		return true
	default:
		return false
	}
}
