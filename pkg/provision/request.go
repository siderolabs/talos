// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provision

import (
	"errors"
	"net/netip"
	"slices"

	"github.com/google/uuid"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

// ClusterRequestBase is the base options common across providers for the cluster to be created.
type ClusterRequestBase struct {
	Name string

	Network       NetworkRequestBase
	Workers       BaseNodeRequests
	Controlplanes BaseNodeRequests

	// Path to talosctl executable to re-execute itself as needed.
	SelfExecutable string

	// Path to root of state directory (~/.talos/clusters by default).
	StateDirectory string
}

// NetworkRequestBase describes cluster network parameters common across OSs and providers.
type NetworkRequestBase struct {
	CIDRs        []netip.Prefix
	Name         string
	MTU          int
	GatewayAddrs []netip.Addr
}

// BaseNodeRequests is a list of NodeRequest.
type BaseNodeRequests []NodeRequestBase

// FindInitNode looks up init node, it returns an error if no init node is present or if it's duplicate.
func (reqs BaseNodeRequests) FindInitNode() (req NodeRequestBase, err error) {
	found := false

	for i := range reqs {
		if reqs[i].Config == nil {
			continue
		}

		if reqs[i].Config.Machine().Type() == machine.TypeInit {
			if found {
				err = errors.New("duplicate init node in requests")

				return
			}

			req = reqs[i]
			found = true
		}
	}

	if !found {
		err = errors.New("no init node found in requests")
	}

	return
}

// ControlPlaneNodes returns subset of nodes which are Init/ControlPlane type.
func (reqs BaseNodeRequests) ControlPlaneNodes() (nodes []NodeRequestBase) {
	for i := range reqs {
		if reqs[i].Type == machine.TypeInit || reqs[i].Type == machine.TypeControlPlane {
			nodes = append(nodes, reqs[i])
		}
	}

	return
}

// WorkerNodes returns subset of nodes which are Init/ControlPlane type.
func (reqs BaseNodeRequests) WorkerNodes() (nodes []NodeRequestBase) {
	for i := range reqs {
		if reqs[i].Type == machine.TypeWorker {
			nodes = append(nodes, reqs[i])
		}
	}

	return
}

// NodeRequestBase describes a request for a node.
type NodeRequestBase struct {
	Name  string
	Type  machine.Type
	Index int

	Config config.Provider

	// Share of CPUs, in 1e-9 fractions
	NanoCPUs int64
	// Memory limit in bytes
	Memory int64

	// SkipInjectingConfig disables reading configuration from http server
	SkipInjectingConfig bool
	IPs                 []netip.Addr
}

// SiderolinkRequest describes a request for SideroLink agent.
type SiderolinkRequest struct {
	WireguardEndpoint string
	APIEndpoint       string
	APICertificate    []byte
	APIKey            []byte
	SinkEndpoint      string
	LogEndpoint       string
	SiderolinkBind    []SiderolinkBind
}

// GetAddr returns the address for the given UUID.
func (sr *SiderolinkRequest) GetAddr(u *uuid.UUID) (netip.Addr, bool) {
	if idx := slices.IndexFunc(sr.SiderolinkBind, func(sb SiderolinkBind) bool { return sb.UUID == *u }); idx != -1 {
		return sr.SiderolinkBind[idx].Addr, true
	}

	return netip.Addr{}, false
}

// SiderolinkBind describes a pair of prebinded UUID->Addr for SideroLink agent.
type SiderolinkBind struct {
	UUID uuid.UUID
	Addr netip.Addr
}
