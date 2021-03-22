// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package nic provides a way to describe and configure a network interface.
package nic

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/jsimonetti/rtnetlink"
	"github.com/jsimonetti/rtnetlink/rtnl"
	"github.com/mdlayher/netlink"
	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/go-retry/retry"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/protobuf/proto"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/vip"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

const (
	// ref: https://tools.ietf.org/html/rfc791

	// MinimumMTU is the lowest allowed MTU for an interface.
	MinimumMTU = 68
	// MaximumMTU is the highest allowed MTU for an interface.
	MaximumMTU = 65536
)

// NetworkInterface provides an abstract configuration representation for a
// network interface.
type NetworkInterface struct {
	Name            string
	Type            int
	Ignore          bool
	Dummy           bool
	Bonded          bool
	Wireguard       bool
	MTU             uint32
	Link            *net.Interface
	SubInterfaces   []*net.Interface
	AddressMethod   []address.Addressing
	BondSettings    *netlink.AttributeEncoder
	Vlans           []*Vlan
	VirtualIP       net.IP
	WireguardConfig *wgtypes.Config

	rtConn   *rtnetlink.Conn
	rtnlConn *rtnl.Conn

	vipController vip.Controller
}

// New returns a NetworkInterface with all of the given setter options applied.
func New(setters ...Option) (*NetworkInterface, error) {
	// Default interface setup
	iface := defaultOptions()

	// Configure interface with any specified options
	var result *multierror.Error
	for _, setter := range setters {
		result = multierror.Append(result, setter(iface))
	}

	// TODO: May need to look at switching this around to filter by Interface.HardwareAddr
	// Ensure we have an interface name defined
	if iface.Name == "" {
		result = multierror.Append(result, errors.New("interface must have a name"))
	}

	// If no addressing methods have been configured, default to DHCP.
	// If VLANs exist do not force DHCP on master device
	if len(iface.AddressMethod) == 0 && len(iface.Vlans) == 0 {
		iface.AddressMethod = append(iface.AddressMethod, &address.DHCP4{}) // TODO: enable DHCPv6 by default?
	}

	// Handle netlink connection
	conn, err := rtnl.Dial(nil)
	if err != nil {
		result = multierror.Append(result, err)

		return nil, result.ErrorOrNil()
	}

	iface.rtnlConn = conn

	// Need rtnetlink for MTU and bond settings
	nlConn, err := rtnetlink.Dial(nil)
	if err != nil {
		result = multierror.Append(result, err)

		return nil, result.ErrorOrNil()
	}

	iface.rtConn = nlConn

	return iface, result.ErrorOrNil()
}

// IsIgnored checks the network interface to see if it should be ignored and not configured.
func (n *NetworkInterface) IsIgnored() bool {
	if n.Ignore || procfs.ProcCmdline().Get(constants.KernelParamNetworkInterfaceIgnore).Contains(n.Name) {
		return true
	}

	return false
}

// Create creates the underlying link if it does not already exist.
func (n *NetworkInterface) Create() error {
	var info *rtnetlink.LinkInfo

	iface, err := net.InterfaceByName(n.Name)
	if err == nil {
		n.Link = iface

		return nil
	}

	switch {
	case n.Bonded:
		info = &rtnetlink.LinkInfo{Kind: "bond"}
	case n.Dummy:
		info = &rtnetlink.LinkInfo{Kind: "dummy"}
	case n.Wireguard:
		info = &rtnetlink.LinkInfo{Kind: "wireguard"}
	default:
		return fmt.Errorf("unknown device type")
	}

	if err = n.createLink(n.Name, info); err != nil {
		return err
	}

	iface, err = net.InterfaceByName(n.Name)

	if err != nil {
		return err
	}

	n.Link = iface

	return nil
}

// CreateSub create VLAN devices that belongs to a master device.
func (n *NetworkInterface) CreateSub(logger *log.Logger) error {
	var info *rtnetlink.LinkInfo

	// Create all the VLAN devices
	for _, vlan := range n.Vlans {
		name := n.Name + "." + strconv.Itoa(int(vlan.ID))
		logger.Printf("setting up %s", name)
		iface, err := net.InterfaceByName(name)

		if err == nil {
			vlan.Link = iface

			continue
		}

		data, err := vlan.VlanSettings.Encode()
		if err != nil {
			logger.Println("failed to encode vlan link parameters: " + err.Error())

			continue
		}

		// Vlan devices needs the master link index
		masterIdx := uint32(n.Link.Index)
		info = &rtnetlink.LinkInfo{Kind: "vlan", Data: data}

		if err = n.createSubLink(name, info, &masterIdx); err != nil {
			logger.Println("failed to create vlan link " + err.Error())

			return err
		}

		iface, err = net.InterfaceByName(name)
		if err != nil {
			logger.Println("failed to get vlan interface ")

			return err
		}

		vlan.Link = iface
	}

	return nil
}

// Configure is used to set the link state and configure any necessary
// bond settings ( ex, mode ).
//nolint:gocyclo
func (n *NetworkInterface) Configure(ctx context.Context) (err error) {
	if n.IsIgnored() {
		return err
	}

	if n.Bonded {
		if err = n.configureBond(n.Link.Index, n.BondSettings); err != nil {
			return err
		}

		bondIndex := proto.Uint32(uint32(n.Link.Index))

		// TODO: Add check if link is already part of a bond
		if err = n.enslaveLink(bondIndex, n.SubInterfaces...); err != nil {
			return err
		}
	}

	if n.Wireguard {
		if err = n.configureWireguard(n.Name, n.WireguardConfig); err != nil {
			return err
		}
	}

	if err = n.rtnlConn.LinkUp(n.Link); err != nil {
		return err
	}

	if err = n.waitForLinkToBeUp(n.Link); err != nil {
		return fmt.Errorf("failed to bring up interface %q: %w", n.Link.Name, err)
	}

	// Create all the VLAN devices
	for _, vlan := range n.Vlans {
		if err = n.rtnlConn.LinkUp(vlan.Link); err != nil {
			return err
		}

		if err = n.waitForLinkToBeUp(vlan.Link); err != nil {
			return fmt.Errorf("failed to bring up interface %q: %w", vlan.Link.Name, err)
		}
	}

	return nil
}

// RunControllers is used to run additional controllers per interface.
func (n *NetworkInterface) RunControllers(ctx context.Context, logger *log.Logger, eg *errgroup.Group) (err error) {
	if n.VirtualIP != nil {
		if n.vipController, err = vip.New(n.VirtualIP.String(), n.Link.Name); err != nil {
			return fmt.Errorf("failed to create the VirtualIP controller for %q on %q: %w", n.VirtualIP, n.Link.Name, err)
		}

		if err = n.vipController.Start(ctx, logger, eg); err != nil {
			return fmt.Errorf("failed to start the VirtualIP controller for %q on %q: %w", n.VirtualIP, n.Link.Name, err)
		}
	}

	return nil
}

func (n *NetworkInterface) waitForLinkToBeUp(linkDev *net.Interface) error {
	// Wait for link to report up
	var link rtnetlink.LinkMessage

	err := retry.Constant(30*time.Second, retry.WithUnits(250*time.Millisecond), retry.WithJitter(50*time.Millisecond)).Retry(func() error {
		var err error
		link, err = n.rtConn.Link.Get(uint32(linkDev.Index))
		if err != nil {
			return retry.UnexpectedError(err)
		}

		if link.Flags&unix.IFF_UP != unix.IFF_UP {
			return retry.ExpectedError(fmt.Errorf("link is not up %s", n.Link.Name))
		}

		if link.Flags&unix.IFF_RUNNING != unix.IFF_RUNNING {
			return retry.ExpectedError(fmt.Errorf("link is not ready %s", n.Link.Name))
		}

		return nil
	})

	return err
}

// Addressing handles the address method for a configured interface ( dhcp/static ).
// This is inclusive of the address itself as well as any defined routes.
func (n *NetworkInterface) Addressing(logger *log.Logger) error {
	if n.IsIgnored() {
		return nil
	}

	for _, method := range n.AddressMethod {
		if err := n.configureInterface(logger, method, n.Link); err != nil {
			// Treat as non fatal error when failing to configure an interface
			continue
		}
	}

	return nil
}

// AddressingSub handles the address method for a configured sub interface ( dhcp/static ).
// This is inclusive of the address itself as well as any defined routes.
func (n *NetworkInterface) AddressingSub(logger *log.Logger) error {
	if n.IsIgnored() {
		return nil
	}

	for _, vlan := range n.Vlans {
		for _, method := range vlan.AddressMethod {
			if err := n.configureInterface(logger, method, vlan.Link); err != nil {
				logger.Println("failed to configure address on vlan link: " + err.Error())
				// Treat as non fatal error when failing to configure an interface
				continue
			}
		}
	}

	return nil
}

// Renew is the mechanism for keeping a dhcp lease active.
func (n *NetworkInterface) Renew(ctx context.Context, logger *log.Logger) {
	for _, method := range n.AddressMethod {
		if method.TTL() == 0 {
			continue
		}

		go n.renew(ctx, logger, method)
	}
}

// renew sets up the looping to ensure we keep the addressing information
// up to date. We attempt to do our first reconfiguration halfway through
// address TTL. If that fails, we'll continue to attempt to retry every
// halflife.
func (n *NetworkInterface) renew(ctx context.Context, logger *log.Logger, method address.Addressing) {
	const minRenewDuration = 5 * time.Second // protect from renewing too often

	renewDuration := method.TTL() / 2

	var err error

	for {
		select {
		case <-time.After(renewDuration):
		case <-ctx.Done():
			return
		}

		if err = n.configureInterface(logger, method, n.Link); err != nil {
			logger.Printf("failure to renew address for %q: %s", n.Name, err)

			renewDuration = (renewDuration / 2)
		} else {
			renewDuration = method.TTL() / 2
		}

		if renewDuration < minRenewDuration {
			renewDuration = minRenewDuration
		}
	}
}

// configureInterface handles the actual address discovery mechanism and
// netlink interaction to configure the interface.
//nolint:gocyclo,cyclop
func (n *NetworkInterface) configureInterface(logger *log.Logger, method address.Addressing, link *net.Interface) error {
	var err error

	discoverErr := method.Discover(context.Background(), logger, link)

	// Set link MTU in any case
	if err = n.setMTU(method.Link().Index, method.MTU()); err != nil {
		return fmt.Errorf("error setting MTU %d on %q: %w", method.MTU(), n.Name, err)
	}

	if discoverErr != nil {
		return discoverErr
	}

	if method.Address() != nil {
		// Check to see if we need to configure the address
		var addrs []*net.IPNet

		addrs, err = n.rtnlConn.Addrs(method.Link(), method.Family())
		if err != nil {
			return err
		}

		addressExists := false

		for _, addr := range addrs {
			if method.Address().String() == addr.String() {
				addressExists = true

				break
			}
		}

		if !addressExists && method.Address() != nil {
			if err = n.rtnlConn.AddrAdd(method.Link(), method.Address()); err != nil {
				switch err := err.(type) {
				case *netlink.OpError:
					if !os.IsExist(err.Err) && err.Err != syscall.ESRCH {
						return fmt.Errorf("error adding address %s on %q: %w", method.Address(), n.Name, err)
					}
				default:
					return fmt.Errorf("failed to add address (already exists) %+v to %s: %v", method.Address(), method.Link().Name, err)
				}
			}
		}
	}

	// Add any routes
	for _, r := range method.Routes() {
		// If gateway/router is 0.0.0.0 we'll set to nil so route scope decision will be correct
		gw := r.Gateway
		if net.IPv4zero.Equal(gw) || net.IPv6zero.Equal(gw) {
			gw = nil
		}

		src := method.Address()
		// if destination is the ipv6 default route,and gateway is LL do not pass a src address to set the default geteway
		if net.IPv6zero.Equal(r.Destination.IP) && gw.IsLinkLocalUnicast() {
			src = nil
		}

		attr := rtnetlink.RouteAttributes{
			Dst:      r.Destination.IP,
			OutIface: uint32(method.Link().Index),
			Priority: r.Metric,
		}

		if gw != nil {
			attr.Gateway = gw
		}

		err = n.rtnlConn.RouteAdd(method.Link(), *r.Destination, gw, rtnl.WithRouteSrc(src), rtnl.WithRouteAttrs(attr))
		if err != nil {
			// ignore "EEXIST" errors for routes which are already present
			if opErr, ok := err.(*netlink.OpError); !ok || !os.IsExist(opErr.Err) {
				return fmt.Errorf("error adding route %s %s on %q: %s", *r.Destination, gw, n.Name, err)
			}
		}
	}

	return nil
}

// Reset removes addressing configuration from a given link.
func (n *NetworkInterface) Reset() {
	var (
		err  error
		link *net.Interface
		nets []*net.IPNet
	)

	link, err = net.InterfaceByName(n.Name)
	if err != nil {
		return
	}

	if nets, err = n.rtnlConn.Addrs(link, 0); err != nil {
		return
	}

	for _, ipnet := range nets {
		if err = n.rtnlConn.AddrDel(link, ipnet); err != nil {
			continue
		}
	}

	//nolint:errcheck
	n.rtnlConn.LinkDown(link)
}
