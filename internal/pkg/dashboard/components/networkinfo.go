// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"net/netip"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/rivo/tview"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/slices"

	"github.com/siderolabs/talos/internal/pkg/dashboard/resourcedata"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type networkInfoData struct {
	addresses   string
	gateway     string
	resolvers   string
	timeservers string

	routeStatusMap map[resource.ID]*network.RouteStatus
	memberMap      map[resource.ID]*cluster.Member
}

// NetworkInfo represents the network info widget.
type NetworkInfo struct {
	tview.TextView

	selectedNode string
	nodeMap      map[string]*networkInfoData
}

// NewNetworkInfo initializes NetworkInfo.
func NewNetworkInfo() *NetworkInfo {
	component := &NetworkInfo{
		TextView: *tview.NewTextView(),
		nodeMap:  make(map[string]*networkInfoData),
	}

	component.SetDynamicColors(true).
		SetText(noData).
		SetBorderPadding(1, 0, 1, 0)

	return component
}

// OnNodeSelect implements the NodeSelectListener interface.
func (widget *NetworkInfo) OnNodeSelect(node string) {
	if node != widget.selectedNode {
		widget.selectedNode = node

		widget.redraw()
	}
}

// OnResourceDataChange implements the ResourceDataListener interface.
func (widget *NetworkInfo) OnResourceDataChange(data resourcedata.Data) {
	widget.updateNodeData(data)

	if data.Node == widget.selectedNode {
		widget.redraw()
	}
}

func (widget *NetworkInfo) updateNodeData(data resourcedata.Data) {
	nodeData := widget.getOrCreateNodeData(data.Node)

	switch res := data.Resource.(type) {
	case *network.ResolverStatus:
		if data.Deleted {
			nodeData.resolvers = notAvailable
		} else {
			nodeData.resolvers = widget.resolvers(res)
		}
	case *network.TimeServerStatus:
		if data.Deleted {
			nodeData.timeservers = notAvailable
		} else {
			nodeData.timeservers = widget.timeservers(res)
		}
	case *network.RouteStatus:
		if data.Deleted {
			delete(nodeData.routeStatusMap, res.Metadata().ID())
		} else {
			nodeData.routeStatusMap[res.Metadata().ID()] = res
		}

		nodeData.gateway = widget.gateway(maps.Values(nodeData.routeStatusMap))
	case *cluster.Member:
		if data.Deleted {
			delete(nodeData.memberMap, res.Metadata().ID())
		} else {
			nodeData.memberMap[res.Metadata().ID()] = res
		}

		nodeData.addresses = widget.addresses(data.Node, maps.Values(nodeData.memberMap))
	}
}

func (widget *NetworkInfo) getOrCreateNodeData(node string) *networkInfoData {
	data, ok := widget.nodeMap[node]
	if !ok {
		data = &networkInfoData{
			addresses:      notAvailable,
			gateway:        notAvailable,
			resolvers:      notAvailable,
			timeservers:    notAvailable,
			routeStatusMap: make(map[resource.ID]*network.RouteStatus),
			memberMap:      make(map[resource.ID]*cluster.Member),
		}

		widget.nodeMap[node] = data
	}

	return data
}

func (widget *NetworkInfo) redraw() {
	data := widget.getOrCreateNodeData(widget.selectedNode)

	fields := fieldGroup{
		fields: []field{
			{
				Name:  "IP",
				Value: data.addresses,
			},
			{
				Name:  "GW",
				Value: data.gateway,
			},
			// TODO: enable when implemented
			// {
			// 	Name:  "OUTBOUND",
			// 	Value: data.outbound,
			// },
			{
				Name:  "DNS",
				Value: data.resolvers,
			},
			{
				Name:  "NTP",
				Value: data.timeservers,
			},
		},
	}

	widget.SetText(fields.String())
}

func (widget *NetworkInfo) addresses(node string, members []*cluster.Member) string {
	var currentMember *cluster.Member

	for _, member := range members {
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

func (widget *NetworkInfo) gateway(statuses []*network.RouteStatus) string {
	resultV4 := notAvailable
	resultV6 := notAvailable

	priorityV4 := uint32(0)
	priorityV6 := uint32(0)

	for _, status := range statuses {
		gateway := status.TypedSpec().Gateway
		if !gateway.IsValid() {
			continue
		}

		if gateway.Is4() && status.TypedSpec().Priority > priorityV4 {
			resultV4 = gateway.String()
			priorityV4 = status.TypedSpec().Priority
		} else if gateway.Is6() && status.TypedSpec().Priority > priorityV6 {
			resultV6 = gateway.String()
			priorityV6 = status.TypedSpec().Priority
		}
	}

	if resultV4 == notAvailable {
		return resultV6
	}

	return resultV4
}

func (widget *NetworkInfo) resolvers(status *network.ResolverStatus) string {
	strs := slices.Map(status.TypedSpec().DNSServers, func(t netip.Addr) string {
		return t.String()
	})

	if len(strs) == 0 {
		return none
	}

	return strings.Join(strs, ", ")
}

func (widget *NetworkInfo) timeservers(status *network.TimeServerStatus) string {
	if len(status.TypedSpec().NTPServers) == 0 {
		return none
	}

	return strings.Join(status.TypedSpec().NTPServers, ", ")
}
