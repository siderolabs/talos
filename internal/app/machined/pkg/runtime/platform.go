// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"

	"github.com/talos-systems/go-procfs/procfs"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// Platform defines the requirements for a platform.
type Platform interface {
	// Name returns platform name.
	Name() string

	// Mode returns platform mode (metal, cloud or container).
	Mode() Mode

	// Configuration fetches the machine configuration from platform-specific location.
	//
	// On cloud-like platform it is user-data in metadata service.
	// For metal platform that is either `talos.config=` URL or mounted ISO image.
	Configuration(context.Context) ([]byte, error)

	// KernelArgs returns additional kernel arguments which should be injected for the kernel boot.
	KernelArgs() procfs.Parameters

	// NetworkConfiguration fetches network configuration from the platform metadata.
	//
	// Controller will run this in function a separate goroutine, restarting it
	// on error. Platform is expected to deliver network configuration over the channel,
	// including updates to the configuration over time.
	NetworkConfiguration(context.Context, chan<- *PlatformNetworkConfig) error
}

// PlatformNetworkConfig describes the network configuration produced by the platform.
//
// This structure is marshaled to STATE partition to persist cached network configuration across
// reboots.
type PlatformNetworkConfig struct {
	Addresses []network.AddressSpecSpec `yaml:"addresses"`
	Links     []network.LinkSpecSpec    `yaml:"links"`
	Routes    []network.RouteSpecSpec   `yaml:"routes"`

	Hostnames   []network.HostnameSpecSpec   `yaml:"hostnames"`
	Resolvers   []network.ResolverSpecSpec   `yaml:"resolvers"`
	TimeServers []network.TimeServerSpecSpec `yaml:"timeServers"`

	Operators []network.OperatorSpecSpec `yaml:"operators"`

	ExternalIPs []netaddr.IP `yaml:"externalIPs"`
}
