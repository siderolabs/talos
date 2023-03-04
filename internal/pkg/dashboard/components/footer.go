// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/siderolabs/gen/maps"
)

// Footer represents the top bar with host info.
type Footer struct {
	tview.TextView

	nodes             []string
	selectedNodeIndex int

	screenKeyToName map[string]string

	lock          sync.Mutex
	currentScreen string
}

// NewFooter initializes Footer.
func NewFooter(screenKeyToName map[string]string) *Footer {
	var initialScreen string
	for _, name := range screenKeyToName {
		initialScreen = name

		break
	}

	widget := &Footer{
		TextView:        *tview.NewTextView(),
		screenKeyToName: screenKeyToName,
		currentScreen:   initialScreen,
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

	widget.Refresh()

	return widget
}

// UpdateNodes updates the nodes list and returns the selected node.
func (widget *Footer) UpdateNodes(nodes []string) string {
	widget.lock.Lock()
	defer widget.lock.Unlock()

	sort.Strings(nodes)

	widget.nodes = nodes

	widget.refresh()

	return widget.selectedNode()
}

// SelectNode selects the node by moving the selected index by the given move.
// Returns the selected node.
func (widget *Footer) SelectNode(move int) string {
	widget.lock.Lock()
	defer widget.lock.Unlock()

	widget.selectedNodeIndex += move

	widget.refresh()

	selectedNode := widget.selectedNode()

	return selectedNode
}

// SelectedNode returns the selected node.
func (widget *Footer) SelectedNode() string {
	widget.lock.Lock()
	defer widget.lock.Unlock()

	return widget.selectedNode()
}

// SelectScreen refreshes the footer with the tabs and screens data.
func (widget *Footer) SelectScreen(screen string) {
	widget.lock.Lock()
	defer widget.lock.Unlock()

	widget.currentScreen = screen

	widget.refresh()
}

// Refresh refreshes the footer with the tabs and screens data.
func (widget *Footer) Refresh() {
	widget.lock.Lock()
	defer widget.lock.Unlock()

	widget.refresh()
}

func (widget *Footer) selectedNode() string {
	if len(widget.nodes) == 0 {
		return ""
	}

	return widget.nodes[widget.selectedNodeIndex]
}

func (widget *Footer) refresh() {
	if len(widget.nodes) == 0 || widget.selectedNodeIndex < 0 {
		widget.selectedNodeIndex = 0
	} else if widget.selectedNodeIndex >= len(widget.nodes) {
		widget.selectedNodeIndex = len(widget.nodes) - 1
	}

	widget.SetText(fmt.Sprintf(
		"[%s] --- %s",
		widget.nodesText(),
		widget.screensText(),
	))
}

func (widget *Footer) nodesText() string {
	nodesCopy := make([]string, 0, len(widget.nodes))

	for _, node := range widget.nodes {
		if node == "" {
			nodesCopy = append(nodesCopy, "(local)")
		} else {
			nodesCopy = append(nodesCopy, node)
		}
	}

	if len(nodesCopy) > widget.selectedNodeIndex {
		nodesCopy[widget.selectedNodeIndex] = fmt.Sprintf("[red]%s[-]", nodesCopy[widget.selectedNodeIndex])
	}

	return strings.Join(nodesCopy, " | ")
}

func (widget *Footer) screensText() string {
	screenKeys := maps.Keys(widget.screenKeyToName)
	sort.Strings(screenKeys)

	screenTexts := make([]string, 0, len(widget.screenKeyToName))

	for _, screenKey := range screenKeys {
		screen := widget.screenKeyToName[screenKey]

		if screen == widget.currentScreen {
			screenTexts = append(screenTexts, fmt.Sprintf("[[red]%s[-]]", screen))
		} else {
			screenTexts = append(screenTexts, fmt.Sprintf("[%s: %s]", screenKey, screen))
		}
	}

	return strings.Join(screenTexts, " --- ")
}
