// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package net

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
)

// IPAddrs finds and returns a list of non-loopback IP addresses of the
// current machine.
func IPAddrs() (ips []net.IP, err error) {
	ips = []net.IP{}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok {
			if ipnet.IP.IsGlobalUnicast() && !ipnet.IP.IsLinkLocalUnicast() {
				ips = append(ips, ipnet.IP)
			}
		}
	}

	return ips, nil
}

// FormatAddress checks that the address has a consistent format.
func FormatAddress(addr string) string {
	addr = strings.Trim(addr, "[]")

	if ip := net.ParseIP(addr); ip != nil {
		// If this is an IPv6 address, encapsulate it in brackets
		if ip.To4() == nil {
			return "[" + ip.String() + "]"
		}

		return ip.String()
	}

	return addr
}

// AddressContainsPort checks to see if the supplied address contains both an address and a port.
// This will not catch every possible permutation, but it is a best-effort routine suitable for prechecking human-interactive parameters.
func AddressContainsPort(addr string) bool {
	if !strings.Contains(addr, ":") {
		return false
	}

	pieces := strings.Split(addr, ":")

	if ip := net.ParseIP(strings.Trim(addr, "[]")); ip != nil {
		return false
	}

	// Check to see if it parses as an IP _without_ the last (presumed) `:port`
	trimmedAddr := strings.TrimSuffix(addr, ":"+pieces[len(pieces)-1])

	if ip := net.ParseIP(strings.Trim(trimmedAddr, "[]")); ip != nil {
		// We appear to have a valid IP followed by `:port`
		return true
	}

	if len(pieces) > 2 {
		// No idea what this is, but it doesn't appear to be addr:port
		return false
	}

	// Looks like it is host:port
	return true
}

// NthIPInNetwork takes an IPNet and returns the nth IP in it.
func NthIPInNetwork(network *net.IPNet, n int) (net.IP, error) {
	ip := network.IP
	dst := make([]byte, len(ip))
	copy(dst, ip)

	for i := 0; i < n; i++ {
		for j := len(dst) - 1; j >= 0; j-- {
			dst[j]++

			if dst[j] > 0 {
				break
			}
		}
	}

	if network.Contains(dst) {
		return dst, nil
	}

	return nil, errors.New("network does not contain enough IPs")
}

// DNSNames returns a default set of machine names. It includes the hostname,
// and FQDN if the kernel domain name is set. If the kernel domain name is not
// set, only the hostname is included in the set.
func DNSNames() (dnsNames []string, err error) {
	var (
		hostname   string
		domainname string
	)

	// Add the hostname.

	if hostname, err = os.Hostname(); err != nil {
		return nil, err
	}

	dnsNames = []string{hostname}

	// Add the domain name if it is set.

	if domainname, err = DomainName(); err != nil {
		return nil, err
	}

	if domainname != "" {
		dnsNames = append(dnsNames, fmt.Sprintf("%s.%s", hostname, domainname))
	}

	return dnsNames, nil
}

// DomainName returns the kernel domain name. If a domain name is not found, an
// empty string is returned.
func DomainName() (domainname string, err error) {
	var b []byte

	if b, err = ioutil.ReadFile("/proc/sys/kernel/domainname"); err != nil {
		return "", err
	}

	domainname = string(b)

	if domainname == "(none)\n" {
		return "", nil
	}

	return strings.TrimSuffix(domainname, "\n"), nil
}

// IsIPv6 indicates whether any IP address within the provided set is an IPv6
// address.
func IsIPv6(addrs ...net.IP) bool {
	for _, a := range addrs {
		if a == nil || a.IsLoopback() || a.IsUnspecified() {
			continue
		}

		if a.To4() == nil {
			if a.To16() != nil {
				return true
			}
		}
	}

	return false
}
