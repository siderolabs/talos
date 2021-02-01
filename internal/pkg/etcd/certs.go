// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"fmt"
	stdlibnet "net"
	"os"
	"time"

	"github.com/talos-systems/crypto/x509"
	"github.com/talos-systems/net"
)

// GeneratePeerCert generates etcd peer certificate and key from etcd CA.
func GeneratePeerCert(etcdCA *x509.PEMEncodedCertificateAndKey) (*x509.PEMEncodedCertificateAndKey, error) {
	ips, err := net.IPAddrs()
	if err != nil {
		return nil, fmt.Errorf("failed to discover IP addresses: %w", err)
	}

	ips = append(ips, stdlibnet.ParseIP("127.0.0.1"))
	if net.IsIPv6(ips...) {
		ips = append(ips, stdlibnet.ParseIP("::1"))
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	dnsNames, err := net.DNSNames()
	if err != nil {
		return nil, fmt.Errorf("failed to get host DNS names: %w", err)
	}

	dnsNames = append(dnsNames, "localhost")

	opts := []x509.Option{
		x509.CommonName(hostname),
		x509.DNSNames(dnsNames),
		x509.IPAddresses(ips),
		x509.NotAfter(time.Now().Add(87600 * time.Hour)),
	}

	ca, err := x509.NewCertificateAuthorityFromCertificateAndKey(etcdCA, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed loading CA from config: %w", err)
	}

	keyPair, err := x509.NewKeyPair(ca, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed generating peer key pair: %w", err)
	}

	return x509.NewCertificateAndKeyFromKeyPair(keyPair), nil
}
