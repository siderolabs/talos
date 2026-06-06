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

	// lastWidth is the inner width captured during the last Draw, used so the
	// event setters can re-render the windowed node list outside of a draw.
	lastWidth int

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

	return widget
}

// Draw renders the footer, windowing the node list to the available width so
// the selected node and the screen tabs always stay visible.
func (widget *Footer) Draw(screen tcell.Screen) {
	// The parent Grid sets the rect before Draw, so the inner width is known here.
	_, _, width, _ := widget.GetInnerRect() //nolint:dogsled

	widget.Render(width)
	widget.TextView.Draw(screen)
}

// OnNodeSelect implements the NodeSelectListener interface.
func (widget *Footer) OnNodeSelect(node string) {
	widget.selectedNode = node

	widget.Render(widget.lastWidth)
}

// SelectScreen refreshes the footer with the tabs and screens data.
func (widget *Footer) SelectScreen(screen string) {
	widget.selectedScreen = screen

	widget.Render(widget.lastWidth)
}

// SetPaused refreshes the footer with the new paused state.
func (widget *Footer) SetPaused(paused bool) {
	widget.paused = paused

	widget.Render(widget.lastWidth)
}

// Visible widths of the fixed footer separators and indicators.
const (
	nodeSepWidth    = 3 // " | " between nodes
	nodesEndWidth   = 6 // "] --- " between the nodes and screens sections
	screenSepWidth  = 5 // " --- " between screens
	leftArrowWidth  = 2 // "‹ " overflow indicator
	rightArrowWidth = 2 // " ›" overflow indicator
)

// Render rebuilds the footer text and the mouse-click hit regions for the given
// inner width.
//
// The footer format is: [node1 | node2 | node3] --- [F1: Screen1] --- [Screen2] --- ...
// where the selected node/screen is highlighted in red.
//
// The screens section (and the PAUSED indicator) is always rendered in full;
// the node list gets the remaining width and is windowed around the selected
// node so it stays visible. "‹"/"›" indicators are shown when nodes are hidden
// to the left/right. Hit regions track the x-offset ranges of each visible node
// and screen label.
//
//nolint:gocyclo
func (widget *Footer) Render(width int) {
	if width <= 0 {
		return
	}

	widget.lastWidth = width

	var sb strings.Builder

	x := 0
	widget.hitRegions = widget.hitRegions[:0]

	// write appends s to the text and advances x by the visible character width.
	write := func(s string, visibleWidth int) {
		sb.WriteString(s)

		x += visibleWidth
	}

	// The screens section is always shown in full; reserve its width up front so
	// the node window can take the rest. The leading "] --- " is part of it.
	screensWidth := nodesEndWidth + widget.screensWidth()

	// Budget for the windowed node list, excluding the opening "[" and the
	// reserved screens section.
	nodeBudget := max(width-1-screensWidth, 0)

	start, end := widget.nodeWindow(nodeBudget)

	// Opening bracket wrapping the nodes section.
	write("[", 1)

	if start > 0 {
		write("‹ ", leftArrowWidth)
	}

	for i := start; i < end; i++ {
		if i > start {
			write(" | ", nodeSepWidth)
		}

		displayName := widget.displayName(i)

		// If a single node still does not fit, truncate it so the screens stay visible.
		if end-start == 1 {
			avail := nodeBudget
			if start > 0 {
				avail -= leftArrowWidth
			}

			if end < len(widget.nodes) {
				avail -= rightArrowWidth
			}

			displayName = truncateName(displayName, avail)
		}

		startX := x
		nameLen := len([]rune(displayName))

		if widget.nodes[i] == widget.selectedNode {
			write(fmt.Sprintf("[red]%s[-]", displayName), nameLen)
		} else {
			write(displayName, nameLen)
		}

		widget.hitRegions = append(widget.hitRegions, hitRegion{
			startX: startX,
			endX:   x,
			node:   widget.nodes[i],
		})
	}

	if end < len(widget.nodes) {
		write(" ›", rightArrowWidth)
	}

	// Closing bracket and separator between nodes and screens.
	write("] --- ", nodesEndWidth)

	for i, screenKey := range widget.sortedScreenKeys() {
		if i > 0 {
			write(" --- ", screenSepWidth)
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
		write(" --- ", screenSepWidth)
		write("[[yellow]PAUSED[-]]", 8)
	}

	widget.SetText(sb.String())
}

// sortedScreenKeys returns the screen keys in stable order.
func (widget *Footer) sortedScreenKeys() []string {
	screenKeys := maps.Keys(widget.screenKeyToName)
	slices.Sort(screenKeys)

	return screenKeys
}

// screensWidth returns the visible width of the screens section (without the
// leading "] --- " joiner) including the PAUSED indicator when paused.
func (widget *Footer) screensWidth() int {
	w := 0

	for i, screenKey := range widget.sortedScreenKeys() {
		if i > 0 {
			w += screenSepWidth
		}

		screenName := widget.screenKeyToName[screenKey]

		if screenName == widget.selectedScreen {
			w += len([]rune(screenName)) + 2
		} else {
			w += len(screenKey) + len([]rune(screenName)) + 4
		}
	}

	if widget.paused {
		w += screenSepWidth + 8
	}

	return w
}

// displayName returns the label shown for the node at index i.
func (widget *Footer) displayName(i int) string {
	if widget.nodes[i] == "" {
		return "(local)"
	}

	return widget.nodes[i]
}

// nodeWidth returns the visible width of the node label at index i.
func (widget *Footer) nodeWidth(i int) int {
	return len([]rune(widget.displayName(i)))
}

// nodeWindow returns the [start, end) range of nodes to render so the selected
// node stays visible within budget, leaving room for the "‹"/"›" overflow
// indicators when nodes are hidden.
//
//nolint:gocyclo
func (widget *Footer) nodeWindow(budget int) (int, int) {
	if len(widget.nodes) == 0 || budget == 0 {
		return 0, 0
	}

	sel := 0

	for i, node := range widget.nodes {
		if node == widget.selectedNode {
			sel = i

			break
		}
	}

	// windowWidth is the visible width of [start, end) including the overflow
	// indicators that would be shown for any hidden nodes.
	windowWidth := func(start, end int) int {
		total := 0

		for i := start; i < end; i++ {
			if i > start {
				total += nodeSepWidth
			}

			total += widget.nodeWidth(i)
		}

		if start > 0 {
			total += leftArrowWidth
		}

		if end < len(widget.nodes) {
			total += rightArrowWidth
		}

		return total
	}

	start, end := sel, sel+1

	// Greedily grow the window, alternating right then left, while it fits.
	for {
		grew := false

		if end < len(widget.nodes) && windowWidth(start, end+1) <= budget {
			end++
			grew = true
		}

		if start > 0 && windowWidth(start-1, end) <= budget {
			start--
			grew = true
		}

		if !grew {
			break
		}
	}

	return start, end
}

// truncateName shortens s to at most max visible runes, appending "…" when cut.
func truncateName(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	r := []rune(s)
	if len(r) <= maxWidth {
		return s
	}

	if maxWidth == 1 {
		return "…"
	}

	return string(r[:maxWidth-1]) + "…"
}
