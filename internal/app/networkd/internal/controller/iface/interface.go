package iface

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/jsimonetti/rtnetlink"
	"github.com/jsimonetti/rtnetlink/rtnl"
	"github.com/mdlayher/netlink"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/route"
)

// Selector describes a set of features used to match a network interface
// Empty strings are ignored, but any defined field will be treated as part of an AND match.
type Selector struct {
	// Name is the name of the network interface.
	Name string

	// MAC is the Media Access Control Address (MAC) of the network interface.
	MAC string
}

// Configuration describes the network configuration settings for an interface.
type Configuration struct {
	// Addresses is the list of IP addresses (with subnet: CIDRs) which should be statically-assigned to this interface.
	Addresses []net.IPNet
	
	// MTU indicates the desired Maximum Transmision Unit to set on the interface.
	// If this value is 0, it will be ignored.
	MTU uint32

	// DHCP indicates whether the DHCP client should be run on this interface.
	// Valid values are 'auto', 'yes', and 'no'.
	// 'auto' means that if there are statically-declared IP addresses, DHCP the DHCP client will not be run, but otherwise will be.
	// The default value is 'auto'.
	DHCP string

	// Routes is the list of Routes which should be bound to the lifecycle of this interface.
	Routes []route.Route
}

// BondConfiguration describes the configuration of a network Bond interface.
// Please see https://www.kernel.org/doc/Documentation/networking/bonding.txt for a description of the bonding options.
type BondConfiguration struct {
	Configuration

	// ComponentInterfaces lists the names of the network interfaces which should compose this Bond.
	ComponentInterfaces []string

	// Bonding options.
	// Please see https://www.kernel.org/doc/Documentation/networking/bonding.txt for a description of the bonding options.

	ADActorSysPrio uint16
	ADActorSystem string
	ADSelect string
	ADUserPortKey uint16
	ARPAllTargets string
	ARPIPTarget []string
	ARPInterval uint32
	ARPValidate string
	AllSlavesActive uint8
	DownDelay uint32
	FailOverMac string
	HashPolicy string
	LACPRate string
	LPInterval uint32
	MIIMon uint32
	MinLinks uint32
	Mode string
	NumPeerNotif uint8
	PacketsPerSlave uint32
	PeerNotifyDelay uint32
	Primary string
	PrimaryReselect string
	ResendIGMP uint32
	TLBDynamicLB uint8
	UpDelay uint32
	UseCarrier bool
}

// BridgeConfiguration describes the configuration parameters of a bridge interface.
type BridgeConfiguration struct {
	Configuration

	// ComponentInterfaces lists the names of the network interface which should be joined to this bridge.
	ComponentInterfaces []string
}

// DummyConfiguration describes the configuration parameters of a Dummy interface.
type DummyConfiguration struct {
	Configuration
}

// VlanConfiguration describes the configuration parameters of a VLAN interface.
type VlanConfiguration struct {
	Configuration

	// Parent indicates the parent interface name.
	Parent string

	// VID is the vlan ID.
	// For normal 802.1q systems, the range is 1-4096.
	// For QinQ (802.1ac), the range is 1-16777216.
	VID uint32

	// Protocol describes the protocol for communicating the VLAN tagging.
	// Valid options are 802.1q and 802.1ac.
	// The default is 802.1q.
	Protocol string
}

type baseInterface struct {
	net.Interface

	// Selectors defines the set of criteria on which this interface should be selected or matched to the kernel's list of network interfaces.
	Selectors []Selector
}

func (i *baseInterface) addIPs(ctx context.Context, list ... net.IPNet) error {
	c, err := rtnl.Dial(nil)
	if err != nil {
		return fmt.Errorf("failed to connect to netlink: %w", err)
	}

	defer c.Close()

	existingAddresses, err := i.Addrs()
	if err != nil {
		return fmt.Errorf("failed to get list of interface addresses: %w", err)
	}

	for _, a := range list {
		var found bool

		for _, refAddr := range existingAddresses {
			if refAddr.String() == a.String() {
				found = true
				break
			}
		}

		if !found {
			err = multierror.Append(err, c.AddrAdd(&i.Interface, &a))
		}
	}

	return err
}

func (i *baseInterface) addRoutes(ctx context.Context, list ...*route.Route) error {
	c, err := rtnetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("failed to connect to netlink: %w", err)
	}

	defer c.Close()

	for _, r := range list {
		routeMessage, err := r.RTNetlink()
		if err != nil {
			return fmt.Errorf("failed to convert route %q via %q on interface %q to an rtnetlink route message: %w", r.Destination.String(), r.Gateway.String(), r.Interface, err)
		}

		if routeMessage == nil {
			continue
		}

		if err = c.Route.Add(routeMessage); err != nil {
			// ignore routes which already exist
			if opErr, ok := err.(*netlink.OpError); !ok || !os.IsExist(opErr.Err) {
				return fmt.Errorf("failed to add route %q via %q on interface %q: %w", r.Destination.String(), r.Gateway.String(), r.Interface, err)
			}
		}
	}
	
	return nil
}

type bondInterface struct {
	baseInterface

	cfg BondConfiguration
}

type bridgeInterface struct {
	baseInterface

	cfg BridgeConfiguration
}

type dummyInterface struct {
	baseInterface

	cfg Configuration
}

type physicalInterface struct {
	baseInterface

	cfg Configuration
}

// Up implements the Interface interface
func (i *physicalInterface) Up(ctx context.Context) error {
	// validate the IP configuration

	// set the link state to be up
}

type vlanInterface struct {
	baseInterface

	cfg VlanConfiguration
}
