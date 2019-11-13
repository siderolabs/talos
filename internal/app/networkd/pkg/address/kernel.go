// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package address

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/pkg/kernel"
)

// Kernel implements the Addressing interface
type Kernel struct {
	Addr              string
	ServerAddress     string
	Gateway           string
	Netmask           string
	FQDN              string
	Device            string
	Auto              string
	PrimaryResolver   string
	SecondaryResolver string
	NTPServer         string
}

// Discover doesnt do anything in the static configuration since all
// the necessary configuration data is supplied via config.
// nolint: gocyclo
func (k *Kernel) Discover(ctx context.Context) error {
	var (
		option *string
		err    error
	)

	if option = kernel.ProcCmdline().Get("ip").First(); option == nil {
		return fmt.Errorf("%s", "no kernel.ip argument supplied, skipping")
	}

	// TODO may need to move this earlier in networkd; if ip.dhcp is specified
	// we shouldnt do anything because we dont know which (all?) interfaces the kernel
	// will autoconfigure
	if *option == "dhcp" {
		return fmt.Errorf("unsupported kernel.ip configuration method: %s", *option)
	}

	// https://www.kernel.org/doc/Documentation/filesystems/nfs/nfsroot.txt
	// ip=<client-ip>:<server-ip>:<gw-ip>:<netmask>:<hostname>:<device>:<autoconf>:<dns0-ip>:<dns1-ip>:<ntp0-ip>
	fields := strings.Split(*option, ":")

	if len(fields) < 4 {
		return fmt.Errorf("invalid kernel option syntax: found %d fields but there must be at least 4", len(fields))
	}

	for idx, field := range fields {
		switch idx {
		case 0:
			k.Addr = field
		case 1:
			k.ServerAddress = field
		case 2:
			k.Gateway = field
		case 3:
			k.Netmask = field
		case 4:
			k.FQDN = field
		case 5:
			k.Device = field
		case 6:
			k.Auto = field
		case 7:
			k.PrimaryResolver = field
		case 8:
			k.SecondaryResolver = field
		case 9:
			k.NTPServer = field
		}
	}

	return err
}

// Name returns back the name of the address method.
func (k *Kernel) Name() string {
	return "kernel"
}

// Address returns the IP address
func (k *Kernel) Address() *net.IPNet {
	return &net.IPNet{
		IP:   net.ParseIP(k.Addr),
		Mask: k.Mask(),
	}
}

// Mask returns the netmask.
func (k *Kernel) Mask() net.IPMask {
	netmask := net.ParseIP(k.Netmask).To4()
	return net.IPv4Mask(netmask[0], netmask[1], netmask[2], netmask[3])
}

// MTU returns the specified MTU.
func (k *Kernel) MTU() uint32 {
	return 1500
}

// TTL returns the address lifetime. Since this is static, there is
// no TTL (0).
func (k *Kernel) TTL() time.Duration {
	return 0
}

// Family qualifies the address as ipv4 or ipv6
func (k *Kernel) Family() int {
	if k.Address().IP.To4() != nil {
		return unix.AF_INET
	}

	return unix.AF_INET6
}

// Scope sets the address scope
func (k *Kernel) Scope() uint8 {
	return unix.RT_SCOPE_UNIVERSE
}

// Routes aggregates the specified routes for a given device configuration
func (k *Kernel) Routes() (routes []*Route) {
	return []*Route{}
}

// Resolvers returns the DNS resolvers
func (k *Kernel) Resolvers() []net.IP {
	resolvers := []net.IP{}

	for _, resolver := range []string{k.PrimaryResolver, k.SecondaryResolver} {
		if addr := net.ParseIP(resolver); addr != nil {
			resolvers = append(resolvers, addr)
		}
	}

	return resolvers
}

// Hostname returns the hostname
func (k *Kernel) Hostname() string {
	return strings.Split(k.FQDN, ".")[0]
}

// Link returns the underlying net.Interface that this address
// method is configured for
func (k Kernel) Link() *net.Interface {
	// Try out some common names and take the first
	// found interface
	if k.Device != "" {
		for _, iface := range []string{"eth0", "eth1", "eno1", "eno2", "bond0"} {
			if _, err := net.InterfaceByName(iface); err == nil {
				k.Device = iface
				break
			}
		}
	}

	iface, err := net.InterfaceByName(k.Device)
	if err != nil {
		return nil
	}

	return iface
}

// Valid denotes if this address method should be used.
func (k *Kernel) Valid() bool {
	return true
}
