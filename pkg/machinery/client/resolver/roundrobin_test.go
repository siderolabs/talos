// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resolver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/client/resolver"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestEnsureEndpointsHavePorts(t *testing.T) {
	endpoints := []string{
		"123.123.123.123",
		"exammple.com:111",
		"234.234.234.234:4000",
		"localhost",
		"localhost:890",
		"2001:db8:0:0:0:ff00:42:8329",
		"www.company.com",
		"[2001:db8:4006:812::200e]:8080",
	}
	expected := []string{
		"123.123.123.123:50000",
		"exammple.com:111",
		"234.234.234.234:4000",
		"localhost:50000",
		"localhost:890",
		"[2001:db8:0:0:0:ff00:42:8329]:50000",
		"www.company.com:50000",
		"[2001:db8:4006:812::200e]:8080",
	}

	actual := resolver.EnsureEndpointsHavePorts(endpoints, constants.ApidPort)

	assert.Equal(t, expected, actual)
}
