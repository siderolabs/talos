// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
)

func TestMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	bundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersionCurrent)
	require.NoError(t, err)

	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.yaml")

	f, err := os.Create(path)
	require.NoError(t, err)

	require.NoError(t, yaml.NewEncoder(f).Encode(bundle))

	require.NoError(t, f.Close())

	bundle2, err := secrets.LoadBundle(path)
	require.NoError(t, err)

	bundle2.Clock = bundle.Clock

	assert.Equal(t, bundle, bundle2)
}

func TestUnmarshalStable(t *testing.T) {
	t.Parallel()

	bundle, err := secrets.LoadBundle("testdata/secrets.yaml")
	require.NoError(t, err)

	assert.NotNil(t, bundle.Certs)
	assert.NotNil(t, bundle.Certs.Etcd)
	assert.NotNil(t, bundle.Certs.K8s)
	assert.NotNil(t, bundle.Certs.K8sAggregator)
	assert.NotNil(t, bundle.Certs.K8sServiceAccount)

	assert.NotNil(t, bundle.Cluster)
	assert.NotEmpty(t, bundle.Cluster.ID)
	assert.NotEmpty(t, bundle.Cluster.Secret)

	assert.NotNil(t, bundle.Secrets)
	assert.NotEmpty(t, bundle.Secrets.BootstrapToken)
	assert.NotEmpty(t, bundle.Secrets.SecretboxEncryptionSecret)

	assert.NotNil(t, bundle.TrustdInfo)
	assert.NotEmpty(t, bundle.TrustdInfo.Token)
}
