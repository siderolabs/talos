// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	stdx509 "crypto/x509"
	"testing"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/etcd"
	etcdresource "github.com/siderolabs/talos/pkg/machinery/resources/etcd"
)

func TestNewClientFromSpec(t *testing.T) {
	t.Parallel()

	ca, err := x509.NewSelfSignedCertificateAuthority(x509.ECDSA(true), x509.NotAfter(time.Now().Add(time.Hour)))
	require.NoError(t, err)

	keyPair, err := x509.NewKeyPair(ca,
		x509.CommonName("talos"),
		x509.NotAfter(time.Now().Add(time.Hour)),
		x509.KeyUsage(stdx509.KeyUsageDigitalSignature),
		x509.ExtKeyUsage([]stdx509.ExtKeyUsage{stdx509.ExtKeyUsageClientAuth}),
	)
	require.NoError(t, err)

	spec := &etcdresource.ClientSpecSpec{
		AcceptedCAs:      []*x509.PEMEncodedCertificate{{Crt: ca.CrtPEM}},
		Certificate:      x509.NewCertificateAndKeyFromKeyPair(keyPair),
		LocalEndpoints:   etcd.LocalEndpoints(),
		ClusterEndpoints: []string{"10.0.0.1:2379", "10.0.0.2:2379"},
	}

	// the client is built from the spec without touching the etcd PKI on disk, and without connecting to etcd
	client, err := etcd.NewClientFromSpec(t.Context(), spec, spec.LocalEndpoints)
	require.NoError(t, err)
	require.NoError(t, client.Close())

	client, err = etcd.NewClusterClientFromSpec(t.Context(), spec)
	require.NoError(t, err)

	require.ElementsMatch(t, spec.ClusterEndpoints, client.Endpoints())
	require.NoError(t, client.Close())

	// the control plane machines are not discovered yet
	_, err = etcd.NewClusterClientFromSpec(t.Context(), &etcdresource.ClientSpecSpec{Certificate: spec.Certificate})
	require.ErrorContains(t, err, "no controlplane endpoints discovered yet")

	// bad credentials
	_, err = etcd.NewClientFromSpec(t.Context(), &etcdresource.ClientSpecSpec{}, spec.LocalEndpoints)
	require.ErrorContains(t, err, "client certificate is missing")

	_, err = etcd.NewClientFromSpec(t.Context(), &etcdresource.ClientSpecSpec{
		AcceptedCAs: []*x509.PEMEncodedCertificate{{Crt: []byte("not a certificate")}},
		Certificate: spec.Certificate,
	}, spec.LocalEndpoints)
	require.ErrorContains(t, err, "error loading etcd CA certificate")
}
