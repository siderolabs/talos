// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
)

func TestBuildDiskExpression(t *testing.T) {
	t.Parallel()

	builder := cel.NewBuilder(celenv.DiskLocator())

	expr := builder.NewSelect(
		builder.NextID(),
		builder.NewIdent(builder.NextID(), "disk"),
		"rotational",
	)

	out, err := builder.ToBooleanExpression(expr)
	require.NoError(t, err)

	assert.Equal(t, "disk.rotational", out.String())
}
