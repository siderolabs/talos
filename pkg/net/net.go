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

// IPAddrs finds and returns a list of non-loopback IPv4 addresses of the
// current machine.
func IPAddrs() (ips []net.IP, err error) {
	ips = []net.IP{}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.IsGlobalUnicast() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP)
			}
		}
	}

	return ips, nil
}

// FormatAddress checks that the address has a consistent format.
func FormatAddress(addr string) string {
	if ip := net.ParseIP(addr); ip != nil {
		// If this is an IPv6 address, encapsulate it in brackets
		if ip.To4() == nil {
			return "[" + ip.String() + "]"
		}

		return ip.String()
	}

	return addr
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
// address
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
