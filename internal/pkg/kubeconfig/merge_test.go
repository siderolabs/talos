// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeconfig_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/talos-systems/talos/internal/pkg/kubeconfig"
)

func TestMerger(t *testing.T) {
	errorAlways := func(kubeconfig.ConfigComponent, string) (kubeconfig.ConflictDecision, error) {
		return "", fmt.Errorf("shouldn't be here")
	}
	renameAlways := func(kubeconfig.ConfigComponent, string) (kubeconfig.ConflictDecision, error) {
		return kubeconfig.RenameDecision, nil
	}
	overwriteAlways := func(kubeconfig.ConfigComponent, string) (kubeconfig.ConflictDecision, error) {
		return kubeconfig.OverwriteDecision, nil
	}

	for _, tt := range []struct {
		name     string
		initial  clientcmdapi.Config
		new      clientcmdapi.Config
		expected clientcmdapi.Config
		options  kubeconfig.MergeOptions
	}{
		{ // MergeIntoEmpty
			name: "MergeIntoEmpty",
			initial: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{},
				Clusters:  map[string]*clientcmdapi.Cluster{},
				Contexts:  map[string]*clientcmdapi.Context{},
			},
			new: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"foo@bar": {
						ClientCertificate: "cert1",
					},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"bar": {
						Server: "example.com",
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"foo@bar": {
						Cluster:  "bar",
						AuthInfo: "foo@bar",
					},
				},
				CurrentContext: "nothing",
			},
			expected: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"foo@bar": {
						ClientCertificate: "cert1",
					},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"bar": {
						Server: "example.com",
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"foo@bar": {
						Cluster:  "bar",
						AuthInfo: "foo@bar",
					},
				},
			},
			options: kubeconfig.MergeOptions{
				ConflictHandler: errorAlways,
				OutputWriter:    os.Stdout,
			},
		},
		{ // MergeClean
			name: "MergeClean",
			initial: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"foo@bar": {
						ClientCertificate: "cert1",
					},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"bar": {
						Server: "example.com",
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"foo@bar": {
						Cluster:  "bar",
						AuthInfo: "foo@bar",
					},
				},
				CurrentContext: "foo@bar",
			},
			new: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"fiz@buzz": {
						ClientCertificate: "cert2",
					},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"buzz": {
						Server: "another.com",
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"fiz@buzz": {
						Cluster:  "buzz",
						AuthInfo: "fiz@buzz",
					},
				},
			},
			expected: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"foo@bar": {
						ClientCertificate: "cert1",
					},
					"fiz@buzz": {
						ClientCertificate: "cert2",
					},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"bar": {
						Server: "example.com",
					},
					"buzz": {
						Server: "another.com",
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"foo@bar": {
						Cluster:  "bar",
						AuthInfo: "foo@bar",
					},
					"fiz@buzz": {
						Cluster:  "buzz",
						AuthInfo: "fiz@buzz",
					},
				},
				CurrentContext: "fiz@buzz",
			},
			options: kubeconfig.MergeOptions{
				ActivateContext: true,
				ConflictHandler: errorAlways,
				OutputWriter:    os.Stdout,
			},
		},
		{ // MergeRename
			name: "MergeRename",
			initial: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"foo@bar": {
						ClientCertificate: "cert1",
					},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"bar": {
						Server: "example.com",
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"foo@bar": {
						Cluster:  "bar",
						AuthInfo: "foo@bar",
					},
				},
				CurrentContext: "foo@bar",
			},
			new: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"foo@bar": {
						ClientCertificate: "cert2",
					},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"bar": {
						Server: "another.com",
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"foo@bar": {
						Cluster:  "bar",
						AuthInfo: "foo@bar",
					},
				},
			},
			expected: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"foo@bar": {
						ClientCertificate: "cert1",
					},
					"foo@bar-1": {
						ClientCertificate: "cert2",
					},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"bar": {
						Server: "example.com",
					},
					"bar-1": {
						Server: "another.com",
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"foo@bar": {
						Cluster:  "bar",
						AuthInfo: "foo@bar",
					},
					"foo@bar-1": {
						Cluster:  "bar-1",
						AuthInfo: "foo@bar-1",
					},
				},
				CurrentContext: "foo@bar",
			},
			options: kubeconfig.MergeOptions{
				ConflictHandler: renameAlways,
				OutputWriter:    os.Stdout,
			},
		},
		{ // MergeOverwrite
			name: "MergeOverwrite",
			initial: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"foo@bar": {
						ClientCertificate: "cert1",
					},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"bar": {
						Server: "example.com",
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"foo@bar": {
						Cluster:  "bar",
						AuthInfo: "foo@bar",
					},
				},
				CurrentContext: "foo@bar",
			},
			new: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"foo@bar": {
						ClientCertificate: "cert2",
					},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"bar": {
						Server: "another.com",
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"foo@bar": {
						Cluster:  "bar",
						AuthInfo: "foo@bar",
					},
				},
			},
			expected: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"foo@bar": {
						ClientCertificate: "cert2",
					},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"bar": {
						Server: "another.com",
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"foo@bar": {
						Cluster:  "bar",
						AuthInfo: "foo@bar",
					},
				},
				CurrentContext: "foo@bar",
			},
			options: kubeconfig.MergeOptions{
				ConflictHandler: overwriteAlways,
				OutputWriter:    os.Stdout,
			},
		},
		{ // MergeEqual
			name: "MergeEqual",
			initial: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"foo@bar": {
						ClientCertificate: "cert1",
					},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"bar": {
						Server: "example.com",
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"foo@bar": {
						Cluster:  "bar",
						AuthInfo: "foo@bar",
					},
				},
				CurrentContext: "foo@bar",
			},
			new: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"foo@bar": {
						ClientCertificate: "cert1",
					},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"bar": {
						Server: "example.com",
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"foo@bar": {
						Cluster:  "bar",
						AuthInfo: "foo@bar",
					},
				},
				CurrentContext: "foo@bar",
			},
			expected: clientcmdapi.Config{
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"foo@bar": {
						ClientCertificate: "cert1",
					},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"bar": {
						Server: "example.com",
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"foo@bar": {
						Cluster:  "bar",
						AuthInfo: "foo@bar",
					},
				},
				CurrentContext: "foo@bar",
			},
			options: kubeconfig.MergeOptions{
				ConflictHandler: errorAlways,
				OutputWriter:    os.Stdout,
			},
		},
	} {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			merger := kubeconfig.Merger(*tt.initial.DeepCopy())

			err := merger.Merge(&tt.new, tt.options)
			require.NoError(t, err)

			assert.Equal(t, tt.expected.Clusters, merger.Clusters)
			assert.Equal(t, tt.expected.AuthInfos, merger.AuthInfos)
			assert.Equal(t, tt.expected.Contexts, merger.Contexts)
			assert.Equal(t, tt.expected.CurrentContext, merger.CurrentContext)
		})
	}
}
