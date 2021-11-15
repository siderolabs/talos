// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

func TestULAPrefix(t *testing.T) {
	assert.Equal(t, "fd7f:175a:b97c:5602::/64", network.ULAPrefix("8XuV9TZHW08DOk3bVxQjH9ih_TBKjnh-j44tsCLSBzo=", network.ULAKubeSpan).String())
}
