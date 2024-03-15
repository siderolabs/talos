// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestRedactSecrets(t *testing.T) {
	input, err := generate.NewInput("test", "https://doesntmatter:6443", constants.DefaultKubernetesVersion)
	require.NoError(t, err)

	container, err := input.Config(machine.TypeControlPlane)
	if err != nil {
		return
	}

	config := container.RawV1Alpha1()

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

	config.Redact(replacement)

	require.Equal(t, replacement, config.Machine().Security().Token())
	require.Equal(t, replacement, string(config.Machine().Security().IssuingCA().Key))
	require.Equal(t, replacement, config.Cluster().Secret())
	require.Equal(t, "***", config.Cluster().Token().Secret())
	require.Equal(t, "", config.Cluster().AESCBCEncryptionSecret())
	require.Equal(t, replacement, config.Cluster().SecretboxEncryptionSecret())
	require.Equal(t, replacement, string(config.Cluster().CA().Key))
	require.Equal(t, replacement, string(config.Cluster().Etcd().CA().Key))
	require.Equal(t, replacement, string(config.Cluster().ServiceAccount().Key))
}
