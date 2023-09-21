// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	stdlibx509 "crypto/x509"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// CertificateGenerator contains etcd certificate options.
type CertificateGenerator struct {
	CA *x509.PEMEncodedCertificateAndKey

	NodeAddresses  *network.NodeAddress
	HostnameStatus *network.HostnameStatus
}

// buildOptions set common certificate options.
func (gen *CertificateGenerator) buildOptions(autoSANs, includeLocalhost bool) []x509.Option {
	addresses := gen.NodeAddresses.TypedSpec().IPs()

	if includeLocalhost {
		addresses = append(addresses, netip.MustParseAddr("127.0.0.1"))

		for _, addr := range addresses {
			if addr.Is6() {
				addresses = append(addresses, netip.MustParseAddr("::1"))

				break
			}
		}
	}

	hostname := gen.HostnameStatus.TypedSpec().Hostname
	dnsNames := gen.HostnameStatus.TypedSpec().DNSNames()

	if includeLocalhost {
		dnsNames = append(dnsNames, "localhost")
	}

	result := []x509.Option{
		x509.NotAfter(time.Now().Add(87600 * time.Hour)),
		x509.KeyUsage(stdlibx509.KeyUsageDigitalSignature | stdlibx509.KeyUsageKeyEncipherment),
	}

	if autoSANs {
		result = append(result,
			x509.CommonName(hostname),
			x509.DNSNames(dnsNames),
			x509.IPAddresses(xslices.Map(addresses, func(addr netip.Addr) net.IP {
				return addr.AsSlice()
			})),
		)
	}

	return result
}

// GeneratePeerCert generates etcd peer certificate and key from etcd CA.
func (gen *CertificateGenerator) GeneratePeerCert() (*x509.PEMEncodedCertificateAndKey, error) {
	opts := gen.buildOptions(true, false)

	opts = append(opts,
		x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
			stdlibx509.ExtKeyUsageServerAuth,
			stdlibx509.ExtKeyUsageClientAuth,
		}),
	)

	ca, err := x509.NewCertificateAuthorityFromCertificateAndKey(gen.CA)
	if err != nil {
		return nil, fmt.Errorf("failed loading CA from config: %w", err)
	}

	keyPair, err := x509.NewKeyPair(ca, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed generating peer key pair: %w", err)
	}

	return x509.NewCertificateAndKeyFromKeyPair(keyPair), nil
}

// GenerateServerCert generates server etcd certificate and key from etcd CA.
func (gen *CertificateGenerator) GenerateServerCert() (*x509.PEMEncodedCertificateAndKey, error) {
	opts := gen.buildOptions(true, true)

	opts = append(opts,
		x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
			stdlibx509.ExtKeyUsageServerAuth,
			stdlibx509.ExtKeyUsageClientAuth,
		}),
	)

	ca, err := x509.NewCertificateAuthorityFromCertificateAndKey(gen.CA)
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
func (gen *CertificateGenerator) GenerateClientCert(commonName string) (*x509.PEMEncodedCertificateAndKey, error) {
	opts := gen.buildOptions(false, false)

	opts = append(opts, x509.CommonName(commonName))
	opts = append(opts,
		x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
			stdlibx509.ExtKeyUsageClientAuth,
		}),
	)

	ca, err := x509.NewCertificateAuthorityFromCertificateAndKey(gen.CA)
	if err != nil {
		return nil, fmt.Errorf("failed loading CA from config: %w", err)
	}

	keyPair, err := x509.NewKeyPair(ca, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed generating client key pair: %w", err)
	}

	return x509.NewCertificateAndKeyFromKeyPair(keyPair), nil
}
