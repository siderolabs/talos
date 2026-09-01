// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	stdx509 "crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

func TestNewAdminCertificateAndKey(t *testing.T) {
	t.Parallel()

	now := time.Now()

	ca, err := secrets.NewTalosCA(now)
	require.NoError(t, err)

	talosCA := &x509.PEMEncodedCertificateAndKey{
		Crt: ca.CrtPEM,
		Key: ca.KeyPEM,
	}

	t.Run("roles land in the Subject Organization", func(t *testing.T) {
		t.Parallel()

		cert, err := secrets.NewAdminCertificateAndKey(now, talosCA, role.MakeSet(role.Reader), time.Hour)
		require.NoError(t, err)

		block, _ := pem.Decode(cert.Crt)
		require.NotNil(t, block)

		parsed, err := stdx509.ParseCertificate(block.Bytes)
		require.NoError(t, err)

		assert.Equal(t, []string{"os:reader"}, parsed.Subject.Organization)
	})

	t.Run("an empty role set is refused", func(t *testing.T) {
		t.Parallel()

		_, err := secrets.NewAdminCertificateAndKey(now, talosCA, role.Zero, time.Hour)
		assert.ErrorContains(t, err, "at least one role is required")
	})
}
