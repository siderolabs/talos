// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/dashboard/components"
	"github.com/siderolabs/talos/internal/pkg/dashboard/resourcedata"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/siderolink"
)

// The dashboard renders through tcell, which never lets a terminal control
// character reach the terminal - it draws runes into cells. What it does read out
// of the text is tview's own markup, so a node putting "[red]" or a region tag
// into any string it serves repaints or hides part of the operator's dashboard.
//
// tview.Escape turns "[red]" into "[red[]", which prints as the literal text.
const (
	payload = "node[red]INJECTED[-]value"
	escaped = "node[red[]INJECTED[-[]value"
)

// assertInert checks that the widget's text carries the payload only in its
// escaped form, i.e. tview will print it rather than act on it.
func assertInert(t *testing.T, text string) {
	t.Helper()

	require.Contains(t, text, "INJECTED", "the payload did not reach the widget, so this test proves nothing")
	assert.Contains(t, text, escaped)
	assert.NotContains(t, strings.ReplaceAll(text, escaped, ""), "[red]")
}

func TestTalosInfoEscapesAPIData(t *testing.T) {
	t.Parallel()

	const node = "10.0.0.1"

	for name, res := range map[string]resourcedata.Data{
		"uuid": {Node: node, Resource: func() *hardware.SystemInformation {
			r := hardware.NewSystemInformation("systeminformation")
			r.TypedSpec().UUID = payload

			return r
		}()},
		"cluster name": {Node: node, Resource: func() *cluster.Info {
			r := cluster.NewInfo()
			r.TypedSpec().ClusterName = payload

			return r
		}()},
		"siderolink host": {Node: node, Resource: func() *siderolink.Status {
			r := siderolink.NewStatus()
			r.TypedSpec().Host = payload
			r.TypedSpec().Connected = true

			return r
		}()},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			widget := components.NewTalosInfo()
			widget.OnNodeSelect(node)
			widget.OnResourceDataChange(res)

			assertInert(t, widget.GetText(false))
		})
	}
}

func TestNetworkInfoEscapesAPIData(t *testing.T) {
	t.Parallel()

	const node = "10.0.0.1"

	hostname := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostname.TypedSpec().Hostname = payload

	timeservers := network.NewTimeServerStatus(network.NamespaceName, network.TimeServerID)
	timeservers.TypedSpec().NTPServers = []string{payload}

	for name, res := range map[string]resourcedata.Data{
		"hostname":    {Node: node, Resource: hostname},
		"ntp servers": {Node: node, Resource: timeservers},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			widget := components.NewNetworkInfo()
			widget.OnNodeSelect(node)
			widget.OnResourceDataChange(res)

			assertInert(t, widget.GetText(false))
		})
	}
}

func TestHeaderEscapesAPIData(t *testing.T) {
	t.Parallel()

	const node = "10.0.0.1"

	hostname := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostname.TypedSpec().Hostname = payload

	versionRes := runtime.NewVersion()
	versionRes.TypedSpec().Version = payload
	versionRes.TypedSpec().Name = payload

	for name, res := range map[string]resourcedata.Data{
		"hostname": {Node: node, Resource: hostname},
		"version":  {Node: node, Resource: versionRes},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			widget := components.NewHeader()
			widget.OnNodeSelect(node)
			widget.OnResourceDataChange(res)

			assertInert(t, widget.GetText(false))
		})
	}
}

// TestFormattersEscapeTheirText: formatStatus and formatText wrap node text in
// tview tags, so they have to escape the text and not the tags.
func TestFormattersEscapeTheirText(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Node[red[]injected[-[]", components.FormatStatus("node[red]injected[-]"))
	assert.Equal(t, "[green]√ node[red[]x[-[][-]", components.FormatText("node[red]x[-]", true))

	// the ordinary values still render exactly as before.
	assert.Equal(t, "[green]√ Running[-]", components.FormatStatus("running"))
	assert.Equal(t, "[red]× Stopped[-]", components.FormatStatus("stopped"))
	assert.Equal(t, "Unknown", components.FormatStatus("unknown"))
}
