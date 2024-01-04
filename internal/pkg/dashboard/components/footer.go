// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/siderolabs/gen/maps"
)

// Footer represents the top bar with host info.
type Footer struct {
	tview.TextView

	selectedNode string
	nodes        []string

	screenKeyToName map[string]string

	selectedScreen string
}

// NewFooter initializes Footer.
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

func (widget *Footer) refresh() {
	widget.SetText(fmt.Sprintf(
		"[%s] --- %s",
		widget.nodesText(),
		widget.screensText(),
	))
}

func (widget *Footer) nodesText() string {
	nodesCopy := make([]string, 0, len(widget.nodes))

	for _, node := range widget.nodes {
		if node == widget.selectedNode {
			name := node
			if name == "" {
				name = "(local)"
			}

			nodesCopy = append(nodesCopy, fmt.Sprintf("[red]%s[-]", name))
		} else {
			nodesCopy = append(nodesCopy, node)
		}
	}

	return strings.Join(nodesCopy, " | ")
}

func (widget *Footer) screensText() string {
	screenKeys := maps.Keys(widget.screenKeyToName)
	sort.Strings(screenKeys)

	screenTexts := make([]string, 0, len(widget.screenKeyToName))

	for _, screenKey := range screenKeys {
		screen := widget.screenKeyToName[screenKey]

		if screen == widget.selectedScreen {
			screenTexts = append(screenTexts, fmt.Sprintf("[[red]%s[-]]", screen))
		} else {
			screenTexts = append(screenTexts, fmt.Sprintf("[%s: %s]", screenKey, screen))
		}
	}

	return strings.Join(screenTexts, " --- ")
}
