// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pcr_test

import (
	"crypto"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/measure/internal/pcr"
)

func TestSign(t *testing.T) {
	t.Parallel()

	pemKey, err := os.ReadFile("../../testdata/pcr-signing-key.pem")
	require.NoError(t, err)

	block, _ := pem.Decode(pemKey)
	require.NotNil(t, block)

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)

	digest, err := hex.DecodeString("e9590019f04a00029bb5ac512c3d3dfff0ec0e66418cfb5035e22313af891d81")
	require.NoError(t, err)

	signature, err := pcr.Sign(digest, crypto.SHA256, key)
	require.NoError(t, err)

	require.Equal(t,
		&pcr.Signature{
			Digest:          "e9590019f04a00029bb5ac512c3d3dfff0ec0e66418cfb5035e22313af891d81",
			SignatureBase64: "Ylam12MOrPQs2m0AsHzRYBjZwYB1B5W0N4Qq62bNjiV4KgQVpwGTnIA0Rgmdaa1bTL+9+7oZ84H1xR0Q248Yd+2P1ZU5KaSysdoi3nlvotRYUq93HQeVjSLe1WUnoZ56EovP47tPuvLqIHmjPYq3V/EVLS6fD3+mXKZr/Q7sdlUjmGtYO5H0rV39C6Oq4Pwk9WJ4oRRKWwCp4KbxOujJ2ANqJl2QdJJA4WSle8+OML+SomelSDCjwt+s+T+0ZUhCY11Els1PtKO55ySU9N67m7wMIAy7aMwF6vbqyRajFDZN8ad7huhXDpwBGBMaEX5ajm2FseUj+h0EYbAm030FwduqZ9WlTMwp9KUx6dK2uOjckKgItBQfVXFoOo8dl4Al9PDktcmuytogI7o1OdzmJAcrb8BiPLLppmNsEgKR+5+poAsSA3Z0dcREiLbvKm10m7mXHGwRg84knZGSrsbHkD9I3ngeOM3JiPLGGCp4nYjBNzKP4jiygTEgEuZ2ueV9PikwlnM5qaDdByIH+0u3LAJubzN2XyI6TGugNRzdvKRIxtl5dSwRoIptiXInN81q6pw2i27YmzvR16tCTxXFRIcHjxpq5Q4KpVohbYhh4kHiWexbqJMpUPoLVEaw+m+Kh7gMvZlud67I6ldRIjDoy/LSdnsXcjpQFkNoF0ZKhX8=", //nolint:lll
		},
		signature,
	)
}
