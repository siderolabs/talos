// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	stdlibx509 "crypto/x509"
	"fmt"
	stdlibnet "net"
	"os"
	"time"

	"github.com/talos-systems/crypto/x509"
	"github.com/talos-systems/net"

	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// NewCommonOptions set common certificate options.
func NewCommonOptions() ([]x509.Option, error) {
	ips, err := net.IPAddrs()
	if err != nil {
		return nil, fmt.Errorf("failed to discover IP addresses: %w", err)
	}

	ips = net.IPFilter(ips, network.NotSideroLinkStdIP)

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

	return []x509.Option{
		x509.CommonName(hostname),
		x509.DNSNames(dnsNames),
		x509.IPAddresses(ips),
		x509.NotAfter(time.Now().Add(87600 * time.Hour)),
		x509.KeyUsage(stdlibx509.KeyUsageDigitalSignature | stdlibx509.KeyUsageKeyEncipherment),
	}, nil
}

// GeneratePeerCert generates etcd peer certificate and key from etcd CA.
//
//nolint:dupl
func GeneratePeerCert(etcdCA *x509.PEMEncodedCertificateAndKey) (*x509.PEMEncodedCertificateAndKey, error) {
	opts, err := NewCommonOptions()
	if err != nil {
		return nil, err
	}

	opts = append(opts,
		x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
			stdlibx509.ExtKeyUsageServerAuth,
			stdlibx509.ExtKeyUsageClientAuth,
		}),
	)

	ca, err := x509.NewCertificateAuthorityFromCertificateAndKey(etcdCA)
	if err != nil {
		return nil, fmt.Errorf("failed loading CA from config: %w", err)
	}

	keyPair, err := x509.NewKeyPair(ca, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed generating peer key pair: %w", err)
	}

	return x509.NewCertificateAndKeyFromKeyPair(keyPair), nil
}

// GenerateCert generates etcd certificate and key from etcd CA.
//
//nolint:dupl
func GenerateCert(etcdCA *x509.PEMEncodedCertificateAndKey) (*x509.PEMEncodedCertificateAndKey, error) {
	opts, err := NewCommonOptions()
	if err != nil {
		return nil, err
	}

	opts = append(opts,
		x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
			stdlibx509.ExtKeyUsageServerAuth,
			stdlibx509.ExtKeyUsageClientAuth,
		}),
	)

	ca, err := x509.NewCertificateAuthorityFromCertificateAndKey(etcdCA)
	if err != nil {
		return nil, fmt.Errorf("failed loading CA from config: %w", err)
	}

	keyPair, err := x509.NewKeyPair(ca, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed generating client key pair: %w", err)
	}

	return x509.NewCertificateAndKeyFromKeyPair(keyPair), nil
}

// GenerateClientCert generates client certificate and key from etcd CA.
func GenerateClientCert(etcdCA *x509.PEMEncodedCertificateAndKey, commonName string) (*x509.PEMEncodedCertificateAndKey, error) {
	opts, err := NewCommonOptions()
	if err != nil {
		return nil, err
	}

	opts = append(opts, x509.CommonName(commonName))
	opts = append(opts,
		x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
			stdlibx509.ExtKeyUsageClientAuth,
		}),
	)

	ca, err := x509.NewCertificateAuthorityFromCertificateAndKey(etcdCA)
	if err != nil {
		return nil, fmt.Errorf("failed loading CA from config: %w", err)
	}

	keyPair, err := x509.NewKeyPair(ca, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed generating client key pair: %w", err)
	}

	return x509.NewCertificateAndKeyFromKeyPair(keyPair), nil
}
