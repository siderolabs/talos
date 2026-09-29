// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package output_test

import (
	"bytes"
	"testing"
	"unicode"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos/output"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// TestNoFormatRendersControlCharacters pins the decision in newWriter about which
// output formats are filtered.
//
// A resource is served by the node, so its ID is node-supplied. The JSON and YAML
// encoders escape control characters themselves; the table and jsonpath writers
// render values verbatim and need the filter. Every format has to come out inert,
// however it gets there.
func TestNoFormatRendersControlCharacters(t *testing.T) {
	t.Parallel()

	// OSC 52 writes the operator's clipboard; the bare CR overwrites the line
	// talosctl printed itself.
	const payload = "cpu\x1b]52;c;RjM=\x07\x1b[2Jjunk\rOVERWRITTEN"

	for _, format := range []string{"table", "yaml", "json", "jsonpath={.metadata.id}"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			out, err := output.NewWriter(format, &buf)
			require.NoError(t, err)

			require.NoError(t, out.WriteResource("10.0.0.1", hardware.NewProcessorInfo(payload), state.Created))
			require.NoError(t, out.Flush())

			assert.NotEmpty(t, buf.String())

			for _, r := range buf.String() {
				assert.True(t, r == '\n' || r == '\t' || unicode.IsPrint(r), "rune %U reached the terminal in %s output", r, format)
			}
		})
	}
}

// TestJSONPathScalarEscaped: the scalar branch of the jsonpath writer prints the
// value with no encoder in front of it, which is the branch the JSON one does not
// cover.
func TestJSONPathScalarEscaped(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	out, err := output.NewWriter("jsonpath={.metadata.id}", &buf)
	require.NoError(t, err)

	require.NoError(t, out.WriteResource("10.0.0.1", hardware.NewProcessorInfo("cpu\x1b[2Jwiped"), state.Created))
	require.NoError(t, out.Flush())

	assert.Equal(t, `cpu\x1b[2Jwiped`+"\n", buf.String())
}

// TestJSONPathPassthrough: an ordinary value is untouched, so `-o jsonpath=` stays
// usable from a script.
func TestJSONPathPassthrough(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	out, err := output.NewWriter("jsonpath={.metadata.id}", &buf)
	require.NoError(t, err)

	require.NoError(t, out.WriteResource("10.0.0.1", hardware.NewProcessorInfo("CPU0"), state.Created))
	require.NoError(t, out.Flush())

	assert.Equal(t, "CPU0\n", buf.String())
}
