// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package operator implements network operators.
package operator

import (
	"context"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// Operator describes common interface of the operators.
type Operator interface {
	Run(ctx context.Context, notifyCh chan<- struct{})

	Prefix() string

	AddressSpecs() []network.AddressSpecSpec
	RouteSpecs() []network.RouteSpecSpec
	LinkSpecs() []network.LinkSpecSpec

	HostnameSpecs() []network.HostnameSpecSpec
	ResolverSpecs() []network.ResolverSpecSpec
	TimeServerSpecs() []network.TimeServerSpecSpec
}
