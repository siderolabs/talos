// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"net/netip"
	"strings"

	"github.com/rivo/tview"
	"github.com/siderolabs/gen/slices"

	"github.com/siderolabs/talos/internal/pkg/dashboard/data"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
)

// NetworkInfo represents the network info widget.
type NetworkInfo struct {
	tview.TextView
}

// NewNetworkInfo initializes NetworkInfo.
func NewNetworkInfo() *NetworkInfo {
	network := &NetworkInfo{
		TextView: *tview.NewTextView(),
	}

	network.SetDynamicColors(true).
		SetText(noData).
		SetBorderPadding(1, 0, 1, 0)

	return network
}

// Update implements the DataWidget interface.
func (widget *NetworkInfo) Update(node string, data *data.Data) {
	nodeData := data.Nodes[node]
	if nodeData == nil {
		widget.SetText(noData)

		return
	}

	fields := fieldGroup{
		fields: []field{
			{
				Name:  "IP",
				Value: widget.addresses(node, nodeData),
			},
			{
				Name:  "GW",
				Value: widget.gateway(nodeData),
			},
			// TODO: enable when implemented
			// {
			// 	Name:  "OUTBOUND",
			// 	Value: outbound,
			// },
			{
				Name:  "DNS",
				Value: widget.resolvers(nodeData),
			},
			{
				Name:  "NTP",
				Value: widget.timeservers(nodeData),
			},
		},
	}

	widget.SetText(fields.String())
}

func (widget *NetworkInfo) addresses(node string, nodeData *data.Node) string {
	var currentMember *cluster.Member

	for _, member := range nodeData.Members {
		for _, address := range member.TypedSpec().Addresses {
			if address.String() == node {
				currentMember = member

				break
			}
		}
	}

	if currentMember == nil {
		return notAvailable
	}

	ipStrs := slices.Map(currentMember.TypedSpec().Addresses, func(t netip.Addr) string {
		return t.String()
	})

	return strings.Join(ipStrs, ", ")
}

func (widget *NetworkInfo) gateway(nodeData *data.Node) string {
	priority := uint32(0)
	result := notAvailable

	for _, status := range nodeData.RouteStatuses {
		if !status.TypedSpec().Gateway.IsValid() {
			continue
		}

		if status.TypedSpec().Priority > priority {
			result = status.TypedSpec().Gateway.String()
			priority = status.TypedSpec().Priority
		}
	}

	return result
}

func (widget *NetworkInfo) resolvers(nodeData *data.Node) string {
	if nodeData.ResolverStatus == nil {
		return notAvailable
	}

	strs := slices.Map(nodeData.ResolverStatus.TypedSpec().DNSServers, func(t netip.Addr) string {
		return t.String()
	})

	if len(strs) == 0 {
		return none
	}

	return strings.Join(strs, ", ")
}

func (widget *NetworkInfo) timeservers(nodeData *data.Node) string {
	if nodeData.TimeServerStatus == nil {
		return notAvailable
	}

	if len(nodeData.TimeServerStatus.TypedSpec().NTPServers) == 0 {
		return none
	}

	return strings.Join(nodeData.TimeServerStatus.TypedSpec().NTPServers, ", ")
}
