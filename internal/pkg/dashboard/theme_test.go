// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard_test

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	// imported for the side effect of applying the terminal-inherited theme to tview.Styles.
	_ "github.com/siderolabs/talos/internal/pkg/dashboard"
	"github.com/siderolabs/talos/internal/pkg/dashboard/components"
)

// themeSafe reports whether a color rendered by the dashboard adapts to the terminal theme.
//
// Colors are theme-safe when they are either the terminal default, or one of the ANSI-16
// palette colors, which tcell emits as palette indices for the terminal to remap. Absolute
// (RGB) colors bypass the palette altogether, and the grayscale entries of the palette
// (black, silver, gray, white) are indistinguishable from a light or a dark background, so
// none of those may be used unconditionally.
func themeSafe(c tcell.Color) bool {
	if c == tcell.ColorDefault || c == tcell.ColorReset {
		return true
	}

	if !c.Valid() || c.IsRGB() {
		return false
	}

	idx := c & 0xFFFFFFFF

	if idx >= 16 {
		return false
	}

	// black, silver, gray, white
	return idx != 0 && idx != 7 && idx != 8 && idx != 15
}

func describeColor(c tcell.Color) string {
	switch {
	case c == tcell.ColorDefault:
		return "default"
	case c == tcell.ColorReset:
		return "reset"
	case c.IsRGB():
		return "rgb" + c.CSS()
	case c.Valid():
		return fmt.Sprintf("palette(%d)", c&0xFFFFFFFF)
	default:
		return fmt.Sprintf("invalid(%d)", uint64(c))
	}
}

// TestStylesInheritTerminalTheme asserts that the global tview theme leaves the foreground
// and background at the terminal defaults.
func TestStylesInheritTerminalTheme(t *testing.T) {
	for name, color := range map[string]tcell.Color{
		"PrimitiveBackgroundColor": tview.Styles.PrimitiveBackgroundColor,
		"PrimaryTextColor":         tview.Styles.PrimaryTextColor,
		"BorderColor":              tview.Styles.BorderColor,
		"TitleColor":               tview.Styles.TitleColor,
		"GraphicsColor":            tview.Styles.GraphicsColor,
	} {
		if color != tcell.ColorDefault {
			t.Errorf("tview.Styles.%s is %s, want the terminal default", name, describeColor(color))
		}
	}
}

// TestComponentsInheritTerminalTheme renders every dashboard component and asserts that it
// only paints colors which follow the terminal's own color scheme, so that the dashboard
// stays readable on both dark and light terminals.
func TestComponentsInheritTerminalTheme(t *testing.T) {
	const width, height = 120, 30

	app := tview.NewApplication()

	for name, primitive := range map[string]tview.Primitive{
		"cpugraph":       components.NewCPUGraph(),
		"cpuinfo":        components.NewCPUInfo(),
		"diagnostics":    components.NewDiagnostics(),
		"disksparkline":  components.NewDiskSparkline(),
		"footer":         components.NewFooter(map[string]string{"F1": "Summary"}, []string{"node1"}),
		"gauges":         components.NewSystemGauges(),
		"header":         components.NewHeader(),
		"horizontalline": components.NewHorizontalLine("LOGS"),
		"kubernetesinfo": components.NewKubernetesInfo(),
		"loadavggraph":   components.NewLoadAvgGraph(),
		"loadavginfo":    components.NewLoadAvgInfo(),
		"logviewer":      components.NewLogViewer(app),
		"meminfo":        components.NewMemInfo(),
		"memgraph":       components.NewMemGraph(),
		"netsparkline":   components.NewNetSparkline(),
		"networkinfo":    components.NewNetworkInfo(),
		"processtable":   components.NewProcessTable(),
		"procsinfo":      components.NewProcsInfo(),
		"talosinfo":      components.NewTalosInfo(),
	} {
		t.Run(name, func(t *testing.T) {
			screen := tcell.NewSimulationScreen("UTF-8")

			if err := screen.Init(); err != nil {
				t.Fatalf("init simulation screen: %v", err)
			}

			t.Cleanup(screen.Fini)

			screen.SetSize(width, height)

			primitive.SetRect(0, 0, width, height)
			primitive.Draw(screen)
			screen.Show()

			cells, w, _ := screen.GetContents()

			reported := map[string]struct{}{}

			for i, cell := range cells {
				fg, bg, _ := cell.Style.Decompose()

				if themeSafe(fg) && themeSafe(bg) {
					continue
				}

				problem := fmt.Sprintf("fg=%s bg=%s", describeColor(fg), describeColor(bg))
				if _, ok := reported[problem]; ok {
					continue
				}

				reported[problem] = struct{}{}

				t.Errorf("row %d, col %d (%q) does not follow the terminal theme: %s",
					i/w, i%w, string(cell.Runes), problem)
			}
		})
	}
}
