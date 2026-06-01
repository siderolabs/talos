// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/siderolabs/gen/maps"
)

// hitRegion represents a clickable region in the footer.
type hitRegion struct {
	startX int
	endX   int
	node   string // non-empty if this is a node region
	screen string // non-empty if this is a screen region
}

// Footer represents the top bar with host info.
type Footer struct {
	tview.TextView

	selectedNode string
	nodes        []string

	screenKeyToName map[string]string

	selectedScreen string
	paused         bool

	hitRegions []hitRegion

	// NodeClick is called when a node label is clicked. May be nil.
	NodeClick func(node string)
	// ScreenClick is called when a screen label is clicked. May be nil.
	ScreenClick func(screen string)
}

// NewFooter initializes Footer.
//
//nolint:gocyclo
func NewFooter(screenKeyToName map[string]string, nodes []string) *Footer {
	var initialScreen string
	for _, name := range screenKeyToName {
		initialScreen = name

		break
	}

	widget := &Footer{
		TextView:        *tview.NewTextView(),
		screenKeyToName: screenKeyToName,
		selectedScreen:  initialScreen,
		nodes:           nodes,
	}

	widget.SetDynamicColors(true)

	// set the background to be a horizontal line
	widget.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		for i := x; i < x+width; i++ {
			for j := y; j < y+height; j++ {
				screen.SetContent(
					i,
					j,
					tview.BoxDrawingsLightHorizontal,
					nil,
					tcell.StyleDefault.Foreground(tcell.ColorWhite),
				)
			}
		}

		return x, y, width, height
	})

	widget.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action != tview.MouseLeftClick {
			return action, event
		}

		mx, _ := event.Position()
		wx, _, _, _ := widget.GetRect()
		clickX := mx - wx

		for _, region := range widget.hitRegions {
			if clickX >= region.startX && clickX < region.endX {
				if region.node != "" && widget.NodeClick != nil {
					widget.NodeClick(region.node)
				} else if region.screen != "" && widget.ScreenClick != nil {
					widget.ScreenClick(region.screen)
				}

				break
			}
		}

		return action, event
	})

	widget.refresh()

	return widget
}

// OnNodeSelect implements the NodeSelectListener interface.
func (widget *Footer) OnNodeSelect(node string) {
	widget.selectedNode = node

	widget.refresh()
}

// SelectScreen refreshes the footer with the tabs and screens data.
func (widget *Footer) SelectScreen(screen string) {
	widget.selectedScreen = screen

	widget.refresh()
}

// SetPaused refreshes the footer with the new paused state.
func (widget *Footer) SetPaused(paused bool) {
	widget.paused = paused

	widget.refresh()
}

// refresh rebuilds the footer text and updates hit regions for mouse click detection.
//
// The footer format is: [node1 | node2 | node3] --- [F1: Screen1] --- [Screen2] --- ...
// where the selected node/screen is highlighted in red.
// Hit regions track the x-offset ranges of each node and screen label.
func (widget *Footer) refresh() {
	var sb strings.Builder

	x := 0
	widget.hitRegions = widget.hitRegions[:0]

	// write appends s to the text and advances x by the visible character width.
	write := func(s string, visibleWidth int) {
		sb.WriteString(s)

		x += visibleWidth
	}

	// Opening bracket wrapping the nodes section.
	write("[", 1)

	for i, node := range widget.nodes {
		if i > 0 {
			write(" | ", 3)
		}

		displayName := node
		if displayName == "" {
			displayName = "(local)"
		}

		startX := x
		nameLen := len([]rune(displayName))

		if node == widget.selectedNode {
			write(fmt.Sprintf("[red]%s[-]", displayName), nameLen)
		} else {
			write(displayName, nameLen)
		}

		widget.hitRegions = append(widget.hitRegions, hitRegion{
			startX: startX,
			endX:   x,
			node:   node,
		})
	}

	// Closing bracket and separator between nodes and screens.
	write("] --- ", 6)

	screenKeys := maps.Keys(widget.screenKeyToName)
	slices.Sort(screenKeys)

	for i, screenKey := range screenKeys {
		if i > 0 {
			write(" --- ", 5)
		}

		screenName := widget.screenKeyToName[screenKey]
		startX := x

		if screenName == widget.selectedScreen {
			// [[red]ScreenName[-]] renders as [ScreenName] (the [[ is an escaped [).
			write(fmt.Sprintf("[[red]%s[-]]", screenName), len([]rune(screenName))+2)
		} else {
			// [F1: ScreenName] is not a tview color tag and renders literally.
			write(fmt.Sprintf("[%s: %s]", screenKey, screenName), len(screenKey)+len([]rune(screenName))+4)
		}

		widget.hitRegions = append(widget.hitRegions, hitRegion{
			startX: startX,
			endX:   x,
			screen: screenName,
		})
	}

	if widget.paused {
		// [[yellow]PAUSED[-]] renders as [PAUSED] — no click region needed.
		write(" --- ", 5)
		write("[[yellow]PAUSED[-]]", 8)
	}

	widget.SetText(sb.String())
}
