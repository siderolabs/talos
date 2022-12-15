// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestRedactSecrets(t *testing.T) {
	bundle, err := generate.NewSecretsBundle(generate.NewClock())
	require.NoError(t, err)

	input, err := generate.NewInput("test", "https://doesntmatter:6443", constants.DefaultKubernetesVersion, bundle)
	require.NoError(t, err)

	config, err := generate.Config(machine.TypeControlPlane, input)
	if err != nil {
		return
	}

	require.NotEmpty(t, config.MachineConfig.MachineToken)
	require.NotEmpty(t, config.MachineConfig.MachineCA.Key)
	require.NotEmpty(t, config.ClusterConfig.ClusterSecret)
	require.NotEmpty(t, config.ClusterConfig.BootstrapToken)
	require.Empty(t, config.ClusterConfig.ClusterAESCBCEncryptionSecret)
	require.NotEmpty(t, config.ClusterConfig.ClusterSecretboxEncryptionSecret)
	require.NotEmpty(t, config.ClusterConfig.ClusterCA.Key)
	require.NotEmpty(t, config.ClusterConfig.EtcdConfig.RootCA.Key)
	require.NotEmpty(t, config.ClusterConfig.ClusterServiceAccount.Key)

	replacement := "**.***"

	configBytesBefore, err := config.Bytes()
	require.NoError(t, err)

	redacted := config.RedactSecrets(replacement)

	configBytesAfter, err := config.Bytes()
	require.NoError(t, err)

	require.Equal(t, string(configBytesBefore), string(configBytesAfter), "original config is modified")

	require.Equal(t, replacement, redacted.Machine().Security().Token())
	require.Equal(t, replacement, string(redacted.Machine().Security().CA().Key))
	require.Equal(t, replacement, redacted.Cluster().Secret())
	require.Equal(t, "***", redacted.Cluster().Token().Secret())
	require.Equal(t, "", redacted.Cluster().AESCBCEncryptionSecret())
	require.Equal(t, replacement, redacted.Cluster().SecretboxEncryptionSecret())
	require.Equal(t, replacement, string(redacted.Cluster().CA().Key))
	require.Equal(t, replacement, string(redacted.Cluster().Etcd().CA().Key))
	require.Equal(t, replacement, string(redacted.Cluster().ServiceAccount().Key))
}
