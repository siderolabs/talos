// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos_test

import (
	"bytes"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

// TestRenderMountsEscapesNodeStrings drives a whole command's render path:
// the tabwriter sits between the command and the escaped output, and
// it consumes a tab or a newline in a value before any writer can see it.
func TestRenderMountsEscapesNodeStrings(t *testing.T) {
	t.Parallel()

	// everything a compromised node controls in this response.
	const (
		clipboard  = "\x1b]52;c;RjMtQ0xJUEJPQVJELUhJSkFDSw==\x07"
		repaint    = "\x1b[2J\x1b[HREPAINTED"
		lineRewind = "junk\rOVERWRITTEN"
		columns    = "a\tb\nNODE\tfake\trow"
	)

	responseChan := make(chan multiplex.Response[*machineapi.MountsResponse], 1)
	responseChan <- multiplex.Response[*machineapi.MountsResponse]{
		Node: "10.0.0.1",
		Payload: &machineapi.MountsResponse{
			Messages: []*machineapi.Mounts{
				{
					Stats: []*machineapi.MountStat{
						{Filesystem: clipboard, MountedOn: repaint, Size: 100, Available: 50},
						{Filesystem: lineRewind, MountedOn: columns, Size: 100, Available: 50},
					},
				},
			},
		},
	}

	close(responseChan)

	var buf bytes.Buffer

	require.NoError(t, talos.RenderMounts(&buf, responseChan))

	out := buf.String()

	for _, r := range out {
		assert.True(t, r == '\n' || unicode.IsPrint(r), "rune %U reached the terminal", r)
	}

	// the escaped text is still there to be read, just inert.
	assert.Contains(t, out, `\x1b]52;c;`)
	assert.Contains(t, out, `\x0dOVERWRITTEN`)

	// a node cannot forge extra rows or columns: one header plus one row per stat.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	assert.Len(t, lines, 3)
	assert.NotContains(t, lines[1], "fake")
}
