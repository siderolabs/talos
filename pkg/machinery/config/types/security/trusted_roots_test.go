// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package security_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
)

//go:embed testdata/trustedrootsconfig.yaml
var expectedTrustedRootsConfigDocument []byte

func TestTrustedRootsMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := security.NewTrustedRootsConfigV1Alpha1()
	cfg.MetaName = "custom-ca"
	cfg.Certificates = "-----BEGIN CERTIFICATE-----\nMIIC0DCCAbigAwIBAgIUI7z\n-----END CERTIFICATE-----"

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedTrustedRootsConfigDocument, marshaled)
}

func TestTrustedConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedTrustedRootsConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &security.TrustedRootsConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       security.TrustedRootsConfig,
		},
		MetaName:     "custom-ca",
		Certificates: "-----BEGIN CERTIFICATE-----\nMIIC0DCCAbigAwIBAgIUI7z\n-----END CERTIFICATE-----",
	}, docs[0])
}

func TestTrustedConfigHeader(t *testing.T) {
	t.Parallel()

	cfg := security.NewTrustedRootsConfigV1Alpha1()
	cfg.MetaName = "custom-ca"
	cfg.Certificates = "-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----"

	assert.Equal(t, []string{"\ncustom-ca:\n==========\n-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----"}, cfg.ExtraTrustedRootCertificates())
}
