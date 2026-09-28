// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/rivo/tview"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/internal/pkg/dashboard/resourcedata"
	"github.com/siderolabs/talos/internal/pkg/dashboard/utils"
	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

const maxRecoveryKeyErrorLength = 160

// RecoveryKeyGrid is the screen to supply the disk encryption recovery key.
//
// It lists the volumes which have a recovery key configured (locked, pending enrollment, or enrolled),
// and lets the operator type in the recovery key to unlock the locked volumes or to enroll the pending ones.
// The screen is selected automatically when a volume becomes locked.
type RecoveryKeyGrid struct {
	tview.Grid

	dashboard *Dashboard

	form        *tview.Form
	volumesView *tview.TextView
	keyField    *tview.InputField
	infoView    *tview.TextView

	selectedNode string
	nodeVolumes  map[string]map[string]*block.VolumeStatusSpec

	// supplied tracks the volumes the key was supplied for, to report the outcome
	supplied map[string]struct{}
	masked   bool
}

// NewRecoveryKeyGrid initializes RecoveryKeyGrid.
func NewRecoveryKeyGrid(ctx context.Context, dashboard *Dashboard) *RecoveryKeyGrid {
	grid := &RecoveryKeyGrid{
		Grid:        *tview.NewGrid(),
		dashboard:   dashboard,
		form:        tview.NewForm(),
		volumesView: tview.NewTextView().SetDynamicColors(true).SetLabel("Volumes").SetSize(8, 0).SetScrollable(false),
		keyField:    tview.NewInputField().SetLabel("Recovery Key").SetMaskCharacter('*'),
		infoView:    tview.NewTextView().SetDynamicColors(true).SetSize(3, 0).SetScrollable(false),

		nodeVolumes: map[string]map[string]*block.VolumeStatusSpec{},
		supplied:    map[string]struct{}{},
		masked:      true,
	}

	grid.SetRows(-1, 22, -1).SetColumns(-1, 80, -1)

	grid.form.SetBorder(true).SetTitle(" Disk Encryption Recovery Key ")

	grid.form.AddFormItem(grid.volumesView)
	grid.form.AddFormItem(grid.keyField)
	grid.form.AddButton("Unlock", func() {
		grid.supply(ctx)
	})
	grid.form.AddButton("Show", func() {
		grid.toggleMask()
	})
	grid.form.AddButton("Clear", func() {
		grid.clearForm()
	})
	grid.form.AddFormItem(grid.infoView)

	grid.AddItem(tview.NewBox(), 0, 0, 1, 3, 0, 0, false)
	grid.AddItem(tview.NewBox(), 1, 0, 1, 1, 0, 0, false)
	grid.AddItem(grid.form, 1, 1, 1, 1, 0, 0, false)
	grid.AddItem(tview.NewBox(), 1, 2, 1, 1, 0, 0, false)
	grid.AddItem(tview.NewBox(), 2, 0, 1, 3, 0, 0, false)

	grid.redraw()

	return grid
}

// OnNodeSelect implements the NodeSelectListener interface.
func (widget *RecoveryKeyGrid) OnNodeSelect(node string) {
	if node != widget.selectedNode {
		widget.selectedNode = node

		widget.clearForm()
		widget.redraw()
		widget.updateVisibility()
	}
}

// OnResourceDataChange implements the ResourceDataListener interface.
func (widget *RecoveryKeyGrid) OnResourceDataChange(data resourcedata.Data) {
	volumeStatus, ok := data.Resource.(*block.VolumeStatus)
	if !ok {
		return
	}

	volumes := widget.nodeVolumes[data.Node]
	if volumes == nil {
		volumes = map[string]*block.VolumeStatusSpec{}
		widget.nodeVolumes[data.Node] = volumes
	}

	if data.Deleted {
		delete(volumes, volumeStatus.Metadata().ID())
	} else {
		volumes[volumeStatus.Metadata().ID()] = volumeStatus.TypedSpec()
	}

	if data.Node != widget.selectedNode {
		return
	}

	widget.reportOutcome(volumeStatus.Metadata().ID(), volumeStatus.TypedSpec())
	widget.redraw()
	widget.updateVisibility()
}

// updateVisibility shows the screen (and selects it) while a volume of the selected node is locked, and hides it otherwise.
func (widget *RecoveryKeyGrid) updateVisibility() {
	if len(widget.lockedVolumes()) > 0 {
		if widget.dashboard.isHidden(ScreenRecoveryKey) {
			widget.dashboard.showScreen(ScreenRecoveryKey)
			widget.dashboard.selectScreen(ScreenRecoveryKey)
		}

		return
	}

	widget.dashboard.hideScreen(ScreenRecoveryKey)
}

// reportOutcome shows what happened to a volume the key was supplied for.
func (widget *RecoveryKeyGrid) reportOutcome(id string, spec *block.VolumeStatusSpec) {
	if _, ok := widget.supplied[id]; !ok {
		return
	}

	switch {
	case spec.Phase != block.VolumePhaseLocked:
		delete(widget.supplied, id)

		widget.infoView.SetText(fmt.Sprintf("[green]%s unlocked with the recovery key[-]", tview.Escape(id)))
	case strings.Contains(spec.ErrorMessage, "encryption key rejected"):
		delete(widget.supplied, id)

		widget.infoView.SetText(fmt.Sprintf("[red]Recovery key rejected for %s, check the key and try again[-]", tview.Escape(id)))

		// put the cursor back into the key field for the retry
		widget.form.SetFocus(0)
		widget.dashboard.app.SetFocus(widget.form)
	}
}

// toggleMask shows or hides the typed key, so it can be checked for typos.
func (widget *RecoveryKeyGrid) toggleMask() {
	widget.masked = !widget.masked

	if widget.masked {
		widget.keyField.SetMaskCharacter('*')
		widget.form.GetButton(1).SetLabel("Show")
	} else {
		widget.keyField.SetMaskCharacter(0)
		widget.form.GetButton(1).SetLabel("Hide")
	}
}

func (widget *RecoveryKeyGrid) onScreenSelect(active bool) {
	if active {
		widget.redraw()
		widget.dashboard.app.SetFocus(widget.form)
		widget.form.SetFocus(1) // the key field
	} else {
		widget.clearForm()
	}
}

// relevantVolumes returns the volumes of the selected node which have a recovery key configured, sorted by ID.
//
// A locked volume always has a recovery key configured, for unlocked volumes the configured key types are known.
func (widget *RecoveryKeyGrid) relevantVolumes() []string {
	var ids []string

	for id, spec := range widget.nodeVolumes[widget.selectedNode] {
		if spec.Phase == block.VolumePhaseLocked || slices.Contains(spec.ConfiguredEncryptionKeys, block.EncryptionKeyRecovery.String()) {
			ids = append(ids, id)
		}
	}

	slices.Sort(ids)

	return ids
}

// lockedVolumes returns the locked volumes of the selected node.
func (widget *RecoveryKeyGrid) lockedVolumes() []string {
	return slices.DeleteFunc(widget.relevantVolumes(), func(id string) bool {
		return widget.nodeVolumes[widget.selectedNode][id].Phase != block.VolumePhaseLocked
	})
}

func (widget *RecoveryKeyGrid) redraw() {
	volumes := widget.relevantVolumes()

	if len(volumes) == 0 {
		widget.volumesView.SetText("[gray]no volumes with a recovery key configured[-]")

		return
	}

	var sb strings.Builder

	for _, id := range volumes {
		spec := widget.nodeVolumes[widget.selectedNode][id]

		fmt.Fprintf(&sb, "[::b]%s[::-]: ", tview.Escape(id))

		switch {
		case spec.Phase == block.VolumePhaseLocked:
			reason := spec.ErrorMessage
			if len(reason) > maxRecoveryKeyErrorLength {
				reason = reason[:maxRecoveryKeyErrorLength] + "..."
			}

			fmt.Fprintf(&sb, "[red]locked[-], waiting for the recovery key\n    [gray]%s[-]\n", tview.Escape(reason))
		case !slices.Contains(spec.EnrolledEncryptionKeys, block.EncryptionKeyRecovery.String()):
			sb.WriteString("[yellow]unlocked, recovery key not enrolled yet[-]\n")
		default:
			sb.WriteString("[green]unlocked, recovery key enrolled[-]\n")
		}
	}

	widget.volumesView.SetText(sb.String())
}

func (widget *RecoveryKeyGrid) clearForm() {
	widget.keyField.SetText("")
	widget.infoView.SetText("")

	if !widget.masked {
		widget.toggleMask()
	}
}

// supply sends the recovery key to the node for the locked and pending volumes.
func (widget *RecoveryKeyGrid) supply(ctx context.Context) {
	key := widget.keyField.GetText()
	if key == "" {
		widget.infoView.SetText("[red]Error: no recovery key entered[-]")

		return
	}

	volumes := widget.lockedVolumes()
	if len(volumes) == 0 {
		widget.infoView.SetText("[red]Error: no locked volumes[-]")

		return
	}

	// the dashboard connects with read-only roles by default, supplying the key needs its own role
	md := metadata.Pairs()
	authz.SetMetadata(md, role.MakeSet(role.RecoveryKeySupplier))

	ctx = metadata.NewOutgoingContext(ctx, md)
	ctx = utils.NodeContext(ctx, widget.selectedNode)

	_, err := widget.dashboard.cli.EncryptionRecoveryKeySupply(ctx, volumes, []byte(key))
	if err != nil {
		widget.infoView.SetText(fmt.Sprintf("[red]Error: %v[-]", tview.Escape(err.Error())))

		return
	}

	for _, id := range volumes {
		widget.supplied[id] = struct{}{}
	}

	widget.keyField.SetText("")

	if !widget.masked {
		widget.toggleMask()
	}

	widget.infoView.SetText(fmt.Sprintf("[yellow]Recovery key supplied for %s, unlocking...[-]", tview.Escape(strings.Join(volumes, ", "))))
}
