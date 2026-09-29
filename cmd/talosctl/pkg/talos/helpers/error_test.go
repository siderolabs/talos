// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers_test

import (
	"errors"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
)

// TestAppendErrorsIsPlainText guards the aggregated message against carrying
// terminal styling: it is escaped before it is printed, so a color escape in it
// would reach the operator as literal text instead of a color.
func TestAppendErrorsIsPlainText(t *testing.T) {
	colorized := color.NoColor
	color.NoColor = false

	t.Cleanup(func() { color.NoColor = colorized })

	err := helpers.AppendErrors(nil, errors.New("node said boom"), errors.New("and again"))

	assert.Equal(t, "2 errors occurred:\n node said boom\n and again", err.Error())
}
