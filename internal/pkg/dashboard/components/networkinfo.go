// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"net/netip"
	"sort"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/rivo/tview"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/internal/pkg/dashboard/resourcedata"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

var (
	zeroPrefix    = netip.Prefix{}
	routedNoK8sID = network.FilteredNodeAddressID(network.NodeAddressRoutedID, k8s.NodeAddressFilterNoK8s)
)

type networkInfoData struct {
	hostname     string
	gateway      string
	connectivity string
	resolvers    string
	timeservers  string

	addresses              string
	nodeAddressRouted      *network.NodeAddress
	nodeAddressRoutedNoK8s *network.NodeAddress

	routeStatusMap map[resource.ID]*network.RouteStatus
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

//nolint:gocyclo
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
	case *network.Status:
		if data.Deleted {
			nodeData.connectivity = notAvailable
		} else {
			nodeData.connectivity = widget.connectivity(res)
		}
	case *network.HostnameStatus:
		if data.Deleted {
			nodeData.hostname = notAvailable
		} else {
			nodeData.hostname = res.TypedSpec().Hostname
		}
	case *network.RouteStatus:
		if data.Deleted {
			delete(nodeData.routeStatusMap, res.Metadata().ID())
		} else {
			nodeData.routeStatusMap[res.Metadata().ID()] = res
		}

		nodeData.gateway = widget.gateway(maps.Values(nodeData.routeStatusMap))
	case *network.NodeAddress:
		widget.setAddresses(data, res)
	}
}

func (widget *NetworkInfo) getOrCreateNodeData(node string) *networkInfoData {
	data, ok := widget.nodeMap[node]
	if !ok {
		data = &networkInfoData{
			hostname:       notAvailable,
			addresses:      notAvailable,
			gateway:        notAvailable,
			connectivity:   notAvailable,
			resolvers:      notAvailable,
			timeservers:    notAvailable,
			routeStatusMap: make(map[resource.ID]*network.RouteStatus),
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
				Name:  "HOST",
				Value: data.hostname,
			},
			{
				Name:  "IP",
				Value: data.addresses,
			},
			{
				Name:  "GW",
				Value: data.gateway,
			},
			{
				Name:  "CONNECTIVITY",
				Value: data.connectivity,
			},
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

func (widget *NetworkInfo) setAddresses(data resourcedata.Data, nodeAddress *network.NodeAddress) {
	nodeData := widget.getOrCreateNodeData(data.Node)

	switch nodeAddress.Metadata().ID() {
	case network.NodeAddressRoutedID:
		if data.Deleted {
			nodeData.nodeAddressRouted = nil
		} else {
			nodeData.nodeAddressRouted = nodeAddress
		}
	case routedNoK8sID:
		if data.Deleted {
			nodeData.nodeAddressRoutedNoK8s = nil
		} else {
			nodeData.nodeAddressRoutedNoK8s = nodeAddress
		}
	}

	formatIPs := func(res *network.NodeAddress) string {
		if res == nil {
			return notAvailable
		}

		strs := xslices.Map(res.TypedSpec().Addresses, func(prefix netip.Prefix) string {
			return prefix.String()
		})

		sort.Strings(strs)

		return strings.Join(strs, ", ")
	}

	// if "routed-no-k8s" is available, use it
	if nodeData.nodeAddressRoutedNoK8s != nil {
		nodeData.addresses = formatIPs(nodeData.nodeAddressRoutedNoK8s)

		return
	}

	// fallback to "routed"
	nodeData.addresses = formatIPs(nodeData.nodeAddressRouted)
}

func (widget *NetworkInfo) gateway(statuses []*network.RouteStatus) string {
	var gatewaysV4, gatewaysV6 []string

	for _, status := range statuses {
		gateway := status.TypedSpec().Gateway
		if !gateway.IsValid() ||
			status.TypedSpec().Destination != zeroPrefix {
			continue
		}

		if gateway.Is4() {
			gatewaysV4 = append(gatewaysV4, gateway.String())
		} else {
			gatewaysV6 = append(gatewaysV6, gateway.String())
		}
	}

	if len(gatewaysV4) == 0 && len(gatewaysV6) == 0 {
		return notAvailable
	}

	sort.Strings(gatewaysV4)
	sort.Strings(gatewaysV6)

	return strings.Join(append(gatewaysV4, gatewaysV6...), ", ")
}

func (widget *NetworkInfo) resolvers(status *network.ResolverStatus) string {
	strs := xslices.Map(status.TypedSpec().DNSServers, netip.Addr.String)

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

func (widget *NetworkInfo) connectivity(status *network.Status) string {
	if status.TypedSpec().ConnectivityReady {
		return "[green]√ OK[-]"
	}

	return "[red]× FAILED[-]"
}
