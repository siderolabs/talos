// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
)

var (
	//go:embed testdata/invalid-secrets.yaml
	invalidSecrets []byte
	//go:embed testdata/secrets.yaml
	validSecrets []byte
)

func TestValidate(t *testing.T) {
	t.Parallel()

	var bundle secrets.Bundle
	require.NoError(t, yaml.Unmarshal(validSecrets, &bundle))
	require.NoError(t, bundle.Validate())

	var invalidBundle secrets.Bundle
	require.NoError(t, yaml.Unmarshal(invalidSecrets, &invalidBundle))
	require.EqualError(t, invalidBundle.Validate(), `6 errors occurred:
	* cluster.secret is required
	* one of [secrets.secretboxencryptionsecret, secrets.aescbcencryptionsecret] is required
	* trustdinfo is required
	* certs.etcd is invalid: failed to parse PEM block
	* certs.k8saggregator is required
	* certs.os is invalid: unsupported key type: "CERTIFICATE"

`)
}
