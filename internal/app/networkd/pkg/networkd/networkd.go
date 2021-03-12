// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package networkd handles the network interface configuration on a host.
// If no configuration is provided, automatic configuration via dhcp will
// be performed on interfaces ( eth, en, bond ).
package networkd

import (
	"context"
	"fmt"
	"log"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/talos-systems/go-procfs/procfs"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Set up default nameservers.
const (
	DefaultPrimaryResolver   = "1.1.1.1"
	DefaultSecondaryResolver = "8.8.8.8"
)

// Networkd provides the high level interaction to configure network interfaces
// on a host system. This currently supports addressing configuration via dhcp
// and/or a specified configuration file.
type Networkd struct {
	Interfaces map[string]*nic.NetworkInterface
	Config     config.Provider

	hostname  string
	resolvers []string

	sync.Mutex
	ready bool

	logger *log.Logger
}

// New takes the supplied configuration and creates an abstract representation
// of all interfaces (as nic.NetworkInterface).
//nolint:gocyclo,cyclop
func New(logger *log.Logger, config config.Provider) (*Networkd, error) {
	var (
		hostname  string
		option    *string
		result    *multierror.Error
		resolvers []string
	)

	netconf := make(map[string][]nic.Option)

	if option = procfs.ProcCmdline().Get("ip").First(); option != nil {
		if name, opts := buildKernelOptions(*option); name != "" {
			netconf[name] = opts
		}
	}

	// Gather settings for all config driven interfaces
	if config != nil {
		logger.Println("parsing configuration file")

		for _, device := range config.Machine().Network().Devices() {
			name, opts, err := buildOptions(logger, device, config.Machine().Network().Hostname())
			if err != nil {
				result = multierror.Append(result, err)

				continue
			}

			if _, ok := netconf[name]; ok {
				netconf[name] = append(netconf[name], opts...)
			} else {
				netconf[name] = opts
			}
		}

		hostname = config.Machine().Network().Hostname()

		if len(config.Machine().Network().Resolvers()) > 0 {
			resolvers = config.Machine().Network().Resolvers()
		}
	}

	logger.Println("discovering local interfaces")

	// Gather already present interfaces
	localInterfaces, err := net.Interfaces()
	if err != nil {
		result = multierror.Append(result, err)

		return &Networkd{}, result.ErrorOrNil()
	}

	// Add locally discovered interfaces to our list of interfaces
	// if they are not already present
	filtered, err := filterInterfaces(logger, localInterfaces)
	if err != nil {
		result = multierror.Append(result, err)

		return &Networkd{}, result.ErrorOrNil()
	}

	for _, device := range filtered {
		if _, ok := netconf[device.Name]; !ok {
			netconf[device.Name] = []nic.Option{nic.WithName(device.Name)}

			// Explicitly ignore bonded interfaces if no configuration was specified
			// This should speed up initial boot times since an unconfigured bond
			// does not provide any value.
			if strings.HasPrefix(device.Name, "bond") {
				netconf[device.Name] = append(netconf[device.Name], nic.WithIgnore())
			}
		}

		// Ensure lo has proper loopback address
		// Ensure MTU for loopback is 64k
		// ref: https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/commit/?id=0cf833aefaa85bbfce3ff70485e5534e09254773
		if strings.HasPrefix(device.Name, "lo") {
			netconf[device.Name] = append(netconf[device.Name], nic.WithAddressing(
				&address.Static{
					CIDR: "127.0.0.1/8",
					Mtu:  nic.MaximumMTU,
				},
			))
		}
	}

	// add local interfaces which were filtered out with Ignore
	for _, device := range localInterfaces {
		if _, ok := netconf[device.Name]; !ok {
			netconf[device.Name] = []nic.Option{nic.WithName(device.Name), nic.WithIgnore()}
		}
	}

	interfaces := make(map[string]*nic.NetworkInterface)

	// Create nic.NetworkInterface representation of the interface
	for ifname, opts := range netconf {
		netif, err := nic.New(opts...)
		if err != nil {
			result = multierror.Append(result, err)

			continue
		}

		interfaces[ifname] = netif
	}

	// Set interfaces that are part of a bond to ignored
	for _, netif := range interfaces {
		if !netif.Bonded {
			continue
		}

		for _, subif := range netif.SubInterfaces {
			if _, ok := interfaces[subif.Name]; !ok {
				result = multierror.Append(result, fmt.Errorf("bond subinterface %s does not exist", subif.Name))

				continue
			}

			interfaces[subif.Name].Ignore = true
		}
	}

	return &Networkd{
		Interfaces: interfaces,
		Config:     config,
		hostname:   hostname,
		resolvers:  resolvers,
		logger:     logger,
	}, result.ErrorOrNil()
}

// Configure handles the lifecycle for an interface. This includes creation,
// configuration, and any addressing that is needed. We care about ordering
// here so that we can ensure any links that make up a bond will be in
// the correct state when we get to bonding configuration.
//
//nolint:gocyclo
func (n *Networkd) Configure(ctx context.Context) (err error) {
	// Configure non-bonded interfaces first so we can ensure basic
	// interfaces exist prior to bonding
	for _, bonded := range []bool{false, true} {
		if bonded {
			n.logger.Println("configuring bonded interfaces")
		} else {
			n.logger.Println("configuring non-bonded interfaces")
		}

		if err = n.configureLinks(ctx, bonded); err != nil {
			// Treat errors as non-fatal
			n.logger.Println(err)
		}
	}

	// prefer resolvers from the configuration
	resolvers := append([]string(nil), n.resolvers...)

	// if no resolvers configured, use addressing method resolvers
	if len(resolvers) == 0 {
		for _, netif := range n.Interfaces {
			for _, method := range netif.AddressMethod {
				if !method.Valid() {
					continue
				}

				for _, resolver := range method.Resolvers() {
					resolvers = append(resolvers, resolver.String())
				}
			}
		}
	}

	// use default resolvers if nothing is configured
	if len(resolvers) == 0 {
		resolvers = append(resolvers, DefaultPrimaryResolver, DefaultSecondaryResolver)
	}

	// Set hostname must be before the resolv configuration
	// so we can ensure the hosts domainname is set properly
	// before we write the search stanza
	if err = n.Hostname(); err != nil {
		return err
	}

	if err = writeResolvConf(n.logger, resolvers); err != nil {
		return err
	}

	n.SetReady()

	return nil
}

// Renew sets up a long running loop to refresh a network interfaces
// addressing configuration. Currently this only applies to interfaces
// configured by DHCP.
func (n *Networkd) Renew(ctx context.Context) {
	for _, iface := range n.Interfaces {
		iface.Renew(ctx, n.logger)
	}
}

// Reset handles removing addresses from previously configured interfaces.
func (n *Networkd) Reset() {
	for _, iface := range n.Interfaces {
		iface.Reset()
	}
}

// RunControllers spins up additional controllers in the errgroup.
func (n *Networkd) RunControllers(ctx context.Context, eg *errgroup.Group) error {
	for _, iface := range n.Interfaces {
		if err := iface.RunControllers(ctx, n.logger, eg); err != nil {
			return err
		}
	}

	return nil
}

// Hostname returns the first hostname found from the addressing methods.
// Create /etc/hosts and set hostname.
// Priority is:
// 1. Config (explicitly defined by the user)
// 2. Kernel arg
// 3. Platform
// 4. DHCP
// 5. Default with the format: talos-<ip addr>.
func (n *Networkd) Hostname() (err error) {
	hostname, domainname, address, err := n.decideHostname()
	if err != nil {
		return err
	}

	if err = writeHosts(hostname, address, n.Config); err != nil {
		return err
	}

	var p runtime.Platform

	p, err = platform.CurrentPlatform()
	if err != nil {
		return err
	}

	// Skip hostname/domainname setting when running in container mode
	if p.Mode() == runtime.ModeContainer {
		return nil
	}

	if err = unix.Sethostname([]byte(hostname)); err != nil {
		return err
	}

	return unix.Setdomainname([]byte(domainname))
}

//nolint:gocyclo
func (n *Networkd) decideHostname() (hostname, domainname string, address net.IP, err error) {
	// Set hostname to default
	address = net.ParseIP("127.0.1.1")
	hostname = fmt.Sprintf("%s-%s", "talos", strings.ReplaceAll(address.String(), ".", "-"))

	// Sort interface names alphabetically so we can ensure parsing order
	interfaceNames := make([]string, 0, len(n.Interfaces))
	for intName := range n.Interfaces {
		interfaceNames = append(interfaceNames, intName)
	}

	sort.Strings(interfaceNames)

	// Loop through address responses and use the first hostname
	// and address response.
outer:
	for _, intName := range interfaceNames {
		iface := n.Interfaces[intName]

		// Skip loopback interface because it will always have
		// a hardcoded hostname of `talos-ip`
		if iface.Link != nil && iface.Link.Flags&net.FlagLoopback != 0 {
			continue
		}

		for _, method := range iface.AddressMethod {
			if !method.Valid() {
				continue
			}

			if method.Hostname() != "" {
				hostname = method.Hostname()

				address = method.Address().IP

				break outer
			}
		}
	}

	// Platform
	var p runtime.Platform

	ctx, ctxCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer ctxCancel()

	p, err = platform.CurrentPlatform()
	if err == nil {
		var pHostname []byte

		if pHostname, err = p.Hostname(ctx); err == nil && string(pHostname) != "" {
			hostname = string(pHostname)
		}
	}

	// Kernel
	if kernelHostname := procfs.ProcCmdline().Get(constants.KernelParamHostname).First(); kernelHostname != nil {
		hostname = *kernelHostname
	}

	// Allow user supplied hostname to win
	if n.hostname != "" {
		hostname = n.hostname
	}

	hostParts := strings.Split(hostname, ".")

	if len(hostParts[0]) > 63 {
		return "", "", net.IP{}, fmt.Errorf("hostname length longer than max allowed (63): %s", hostParts[0])
	}

	if len(hostname) > 253 {
		return "", "", net.IP{}, fmt.Errorf("hostname fqdn length longer than max allowed (253): %s", hostname)
	}

	hostname = hostParts[0]

	if len(hostParts) > 1 {
		domainname = strings.Join(hostParts[1:], ".")
	}

	// Only return the hostname portion of the name ( strip domain bits off )
	return hostname, domainname, address, nil
}

// Ready exposes the readiness state of networkd.
func (n *Networkd) Ready() bool {
	n.Lock()
	defer n.Unlock()

	return n.ready
}

// SetReady sets the readiness state of networkd.
func (n *Networkd) SetReady() {
	n.Lock()
	defer n.Unlock()

	n.ready = true
}

func (n *Networkd) configureLinks(ctx context.Context, bonded bool) error {
	errCh := make(chan error, len(n.Interfaces))
	count := 0

	for _, iface := range n.Interfaces {
		if iface.Bonded != bonded {
			continue
		}

		count++

		go func(netif *nic.NetworkInterface) {
			if !netif.IsIgnored() {
				n.logger.Printf("setting up %s", netif.Name)
			}

			errCh <- func() error {
				// Ensure link exists
				if err := netif.Create(); err != nil {
					return fmt.Errorf("error creating nic %q: %w", netif.Name, err)
				}

				if err := netif.CreateSub(n.logger); err != nil {
					return fmt.Errorf("error creating sub interface nic %q: %w", netif.Name, err)
				}

				if err := netif.Configure(ctx); err != nil {
					return fmt.Errorf("error configuring nic %q: %w", netif.Name, err)
				}

				if err := netif.Addressing(n.logger); err != nil {
					return fmt.Errorf("error configuring addressing %q: %w", netif.Name, err)
				}

				if err := netif.AddressingSub(n.logger); err != nil {
					return fmt.Errorf("error configuring addressing %q: %w", netif.Name, err)
				}

				return nil
			}()
		}(iface)
	}

	var multiErr *multierror.Error

	for i := 0; i < count; i++ {
		multiErr = multierror.Append(multiErr, <-errCh)
	}

	return multiErr.ErrorOrNil()
}
