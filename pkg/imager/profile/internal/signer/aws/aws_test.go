// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package aws_test

import (
	"crypto/sha256"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager/profile/internal/signer/aws"
)

func TestIntegration(t *testing.T) {
	for _, envVar := range []string{"AWS_KMS_KEY_ID", "AWS_REGION", "AWS_CERT_PATH"} {
		if os.Getenv(envVar) == "" {
			t.Skipf("%s not set", envVar)
		}
	}

	signer, err := aws.NewPCRSigner(t.Context(), os.Getenv("AWS_KMS_KEY_ID"), os.Getenv("AWS_REGION"))
	require.NoError(t, err)

	digest := sha256.Sum256(nil)

	_, err = signer.Sign(nil, digest[:], nil)
	require.NoError(t, err)

	sbSigner, err := aws.NewSecureBootSigner(t.Context(), os.Getenv("AWS_KMS_KEY_ID"), os.Getenv("AWS_REGION"), os.Getenv("AWS_CERT_PATH"))
	require.NoError(t, err)

	_, err = sbSigner.Signer().Sign(nil, digest[:], nil)
	require.NoError(t, err)
}
