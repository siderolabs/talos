// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components_test

import (
	"strings"
	"testing"

	"github.com/siderolabs/talos/internal/pkg/dashboard/components"
)

func testFooter(nodes []string) *components.Footer {
	f := components.NewFooter(map[string]string{"F1": "Summary", "F2": "Monitor"}, nodes)
	f.SelectScreen("Summary")

	return f
}

func manyNodes(n int) []string {
	nodes := make([]string, n)
	for i := range nodes {
		nodes[i] = "10.0.0." + string(rune('0'+i%10)) + strings.Repeat("x", i/10)
	}

	return nodes
}

func TestFooterRenderFitsAllNodes(t *testing.T) {
	nodes := manyNodes(12)
	f := testFooter(nodes)
	f.OnNodeSelect(nodes[0])

	// Plenty of width: everything fits, no overflow indicators.
	f.Render(400)

	text := f.GetText(true)

	for _, node := range nodes {
		if !strings.Contains(text, node) {
			t.Errorf("expected all nodes visible, missing %q in %q", node, text)
		}
	}

	if strings.ContainsAny(text, "‹›") {
		t.Errorf("did not expect overflow indicators when everything fits: %q", text)
	}

	if !strings.Contains(text, "Monitor") {
		t.Errorf("expected screen tabs visible: %q", text)
	}
}

func TestFooterRenderWindowsAroundSelected(t *testing.T) {
	nodes := manyNodes(12)
	f := testFooter(nodes)

	// First node selected in a narrow terminal: window starts at the left, so a
	// right indicator must appear but not a left one, and the selected node and
	// the screen tabs must remain visible.
	f.OnNodeSelect(nodes[0])
	f.Render(80)

	text := f.GetText(true)

	if !strings.Contains(text, nodes[0]) {
		t.Errorf("expected selected node %q visible: %q", nodes[0], text)
	}

	if !strings.Contains(text, "›") {
		t.Errorf("expected right overflow indicator: %q", text)
	}

	if strings.Contains(text, "‹") {
		t.Errorf("did not expect left overflow indicator at the start: %q", text)
	}

	if !strings.Contains(text, "Monitor") {
		t.Errorf("expected screen tabs always visible: %q", text)
	}

	// Last node selected: the window scrolls right, so a left indicator appears
	// but not a right one, and the selected node stays visible.
	last := nodes[len(nodes)-1]
	f.OnNodeSelect(last)
	f.Render(80)

	text = f.GetText(true)

	if !strings.Contains(text, last) {
		t.Errorf("expected selected last node %q visible: %q", last, text)
	}

	if !strings.Contains(text, "‹") {
		t.Errorf("expected left overflow indicator after scrolling right: %q", text)
	}

	if strings.Contains(text, "›") {
		t.Errorf("did not expect right overflow indicator at the end: %q", text)
	}

	if !strings.Contains(text, "Monitor") {
		t.Errorf("expected screen tabs always visible: %q", text)
	}
}

func TestFooterRenderTruncatesSingleNode(t *testing.T) {
	nodes := []string{"a-very-long-node-name-that-cannot-fit-in-a-narrow-terminal"}
	f := testFooter(nodes)
	f.OnNodeSelect(nodes[0])

	f.Render(60)

	text := f.GetText(true)

	if !strings.Contains(text, "…") {
		t.Errorf("expected the long node name to be truncated with an ellipsis: %q", text)
	}

	if !strings.Contains(text, "Monitor") {
		t.Errorf("expected screen tabs to stay visible: %q", text)
	}
}
