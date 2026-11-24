// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	"net/netip"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestCertificatesIssuedBy(t *testing.T) {
	t.Parallel()

	newCA := func() *x509.PEMEncodedCertificateAndKey {
		ca, err := x509.NewSelfSignedCertificateAuthority(x509.ECDSA(true))
		require.NoError(t, err)

		return &x509.PEMEncodedCertificateAndKey{
			Crt: ca.CrtPEM,
			Key: ca.KeyPEM,
		}
	}

	nodeAddresses := network.NewNodeAddress(network.NamespaceName, network.NodeAddressAccumulativeID)
	nodeAddresses.TypedSpec().Addresses = []netip.Prefix{netip.MustParsePrefix("10.0.0.1/24")}

	hostnameStatus := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostnameStatus.TypedSpec().Hostname = "node1"

	issue := func(ca *x509.PEMEncodedCertificateAndKey) (*x509.PEMEncodedCertificateAndKey, *x509.PEMEncodedCertificateAndKey) {
		generator := etcd.CertificateGenerator{
			CA:             ca,
			NodeAddresses:  nodeAddresses,
			HostnameStatus: hostnameStatus,
		}

		peer, err := generator.GeneratePeerCert()
		require.NoError(t, err)

		client, err := generator.GenerateClientCert("talos")
		require.NoError(t, err)

		return peer, client
	}

	oldCA, newCA1 := newCA(), newCA()

	oldPeer, oldClient := issue(oldCA)
	newPeer, newClient := issue(newCA1)

	for _, test := range []struct {
		name     string
		ca       *x509.PEMEncodedCertificateAndKey
		certs    []*x509.PEMEncodedCertificateAndKey
		expected bool
	}{
		{
			name:     "issued by the CA",
			ca:       oldCA,
			certs:    []*x509.PEMEncodedCertificateAndKey{oldPeer, oldClient},
			expected: true,
		},
		{
			name:  "issuing CA replaced",
			ca:    newCA1,
			certs: []*x509.PEMEncodedCertificateAndKey{oldPeer, oldClient},
		},
		{
			name:  "partially re-issued",
			ca:    newCA1,
			certs: []*x509.PEMEncodedCertificateAndKey{newPeer, oldClient},
		},
		{
			name:     "re-issued",
			ca:       newCA1,
			certs:    []*x509.PEMEncodedCertificateAndKey{newPeer, newClient},
			expected: true,
		},
		{
			name:  "missing certificate",
			ca:    newCA1,
			certs: []*x509.PEMEncodedCertificateAndKey{newPeer, nil},
		},
		{
			name:  "missing CA",
			certs: []*x509.PEMEncodedCertificateAndKey{newPeer},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			issued, err := etcd.CertificatesIssuedBy(test.ca, test.certs...)
			require.NoError(t, err)

			assert.Equal(t, test.expected, issued)
		})
	}

	_, err := etcd.CertificatesIssuedBy(oldCA, &x509.PEMEncodedCertificateAndKey{Crt: []byte("not a certificate")})
	require.Error(t, err)
}
