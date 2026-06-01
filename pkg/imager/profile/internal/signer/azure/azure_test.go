// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package azure_test

import (
	"crypto/sha256"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager/profile/internal/signer/azure"
)

func TestIntegration(t *testing.T) {
	for _, envVar := range []string{"AZURE_VAULT_URL", "AZURE_KEY_ID", "AZURE_CERT_ID", "AZURE_TENANT_ID", "AZURE_CLIENT_ID", "AZURE_CLIENT_SECRET"} {
		if os.Getenv(envVar) == "" {
			t.Skipf("%s not set", envVar)
		}
	}

	signer, err := azure.NewPCRSigner(t.Context(), os.Getenv("AZURE_VAULT_URL"), os.Getenv("AZURE_KEY_ID"), "")
	require.NoError(t, err)

	digest := sha256.Sum256(nil)

	_, err = signer.Sign(nil, digest[:], nil)
	require.NoError(t, err)

	sbSigner, err := azure.NewSecureBootSigner(t.Context(), os.Getenv("AZURE_VAULT_URL"), os.Getenv("AZURE_CERT_ID"), "")
	require.NoError(t, err)

	_, err = sbSigner.Signer().Sign(nil, digest[:], nil)
	require.NoError(t, err)
}
