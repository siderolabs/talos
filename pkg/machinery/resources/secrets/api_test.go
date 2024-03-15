// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

func TestAPIProtobufMarshal(t *testing.T) {
	r := secrets.NewAPI()
	r.TypedSpec().AcceptedCAs = []*x509.PEMEncodedCertificate{
		{
			Crt: []byte("foo"),
		},
	}
	r.TypedSpec().Client = &x509.PEMEncodedCertificateAndKey{
		Crt: []byte("bar"),
		Key: []byte("baz"),
	}
	r.TypedSpec().Server = &x509.PEMEncodedCertificateAndKey{
		Crt: []byte("car"),
		Key: []byte("caz"),
	}

	protoR, err := protobuf.FromResource(r)
	require.NoError(t, err)

	marshaled, err := protoR.Marshal()
	require.NoError(t, err)

	protoR, err = protobuf.Unmarshal(marshaled)
	require.NoError(t, err)

	r2, err := protobuf.UnmarshalResource(protoR)
	require.NoError(t, err)

	require.True(t, resource.Equal(r, r2))
}
