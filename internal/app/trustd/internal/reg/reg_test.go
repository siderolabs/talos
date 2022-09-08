// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package reg_test

import (
	"context"
	stdx509 "crypto/x509"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/peer"

	"github.com/talos-systems/talos/internal/app/trustd/internal/reg"
	"github.com/talos-systems/talos/pkg/machinery/api/security"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/machinery/resources/secrets"
	"github.com/talos-systems/talos/pkg/machinery/role"
)

func TestCertificate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resources := state.WrapCore(namespaced.NewState(inmem.Build))

	ca, err := generate.NewTalosCA(time.Now())
	require.NoError(t, err)

	osRoot := secrets.NewOSRoot(secrets.OSRootID)
	osRoot.TypedSpec().CA = &x509.PEMEncodedCertificateAndKey{
		Crt: ca.CrtPEM,
		Key: ca.KeyPEM,
	}
	require.NoError(t, resources.Create(ctx, osRoot))

	ctx = peer.NewContext(ctx, &peer.Peer{
		Addr: &net.TCPAddr{
			IP:   netip.MustParseAddr("127.0.0.1").AsSlice(),
			Port: 30000,
		},
	})

	r := &reg.Registrator{
		Resources: resources,
	}

	for _, tt := range []struct {
		name       string
		csrSetters []x509.Option
	}{
		{
			name: "server certificate",
			csrSetters: []x509.Option{
				x509.IPAddresses([]net.IP{netip.MustParseAddr("10.5.0.4").AsSlice()}),
				x509.DNSNames([]string{"talos-default-worker-1"}),
				x509.CommonName("talos-default-worker-1"),
			},
		},
		{
			name: "attempt at client certificate",
			csrSetters: []x509.Option{
				x509.CommonName("talos-default-worker-1"),
				x509.Organization(string(role.Impersonator)),
			},
		},
	} {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			serverCSR, serverCert, err := x509.NewEd25519CSRAndIdentity(tt.csrSetters...)
			require.NoError(t, err)

			resp, err := r.Certificate(ctx, &security.CertificateRequest{
				Csr: serverCSR.X509CertificateRequestPEM,
			})
			require.NoError(t, err)

			assert.Equal(t, resp.Ca, ca.CrtPEM)

			serverCert.Crt = resp.Crt

			cert, err := serverCert.GetCert()
			require.NoError(t, err)

			assert.Equal(t, stdx509.KeyUsageDigitalSignature, cert.KeyUsage)
			assert.Equal(t, []stdx509.ExtKeyUsage{stdx509.ExtKeyUsageServerAuth}, cert.ExtKeyUsage)
			assert.Equal(t, "talos-default-worker-1", cert.Subject.CommonName)
			assert.Equal(t, []string(nil), cert.Subject.Organization)
		})
	}
}
