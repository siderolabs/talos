// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ipmi_test

import (
	"testing"

	"github.com/olekukonko/tablewriter/pkg/twwidth"
	"github.com/stretchr/testify/assert"

	_ "github.com/siderolabs/talos/internal/pkg/ipmi"
)

func TestTablewriterCacheDisabled(t *testing.T) {
	t.Parallel()

	_, capacity, _ := twwidth.GetCacheStats()

	assert.Zero(t, capacity)
}
