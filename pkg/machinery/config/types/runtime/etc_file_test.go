// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
)

//go:embed testdata/etc_file.yaml
var expectedEtcFileDocument []byte

func TestEtcFileMarshalStability(t *testing.T) {
	cfg := runtime.NewEtcFileConfigV1Alpha1("nfsmount.conf")
	cfg.Contents = "[NFSMount_Global_Options]\n"

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedEtcFileDocument, marshaled)
}

func TestEtcFileValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		configName    string
		expectedError string
	}{
		{
			name:       "valid",
			configName: "nfsmount.conf",
		},
		{
			name:       "valid nested",
			configName: "foo/bar.conf",
		},
		{
			name:          "empty",
			configName:    "",
			expectedError: "user etc file name cannot be empty",
		},
		{
			name:          "resolv.conf",
			configName:    "resolv.conf",
			expectedError: `user etc file "resolv.conf" is managed by Talos`,
		},
		{
			name:          "hosts",
			configName:    "hosts",
			expectedError: `user etc file "hosts" is managed by Talos`,
		},
		{
			name:          "machine-id",
			configName:    "machine-id",
			expectedError: `user etc file "machine-id" is managed by Talos`,
		},
		{
			name:          "extensions.yaml",
			configName:    "extensions.yaml",
			expectedError: `user etc file "extensions.yaml" is managed by Talos`,
		},
		{
			name:          "os-release",
			configName:    "os-release",
			expectedError: `user etc file "os-release" is managed by Talos`,
		},
		{
			name:          "localtime",
			configName:    "localtime",
			expectedError: `user etc file "localtime" is managed by Talos`,
		},
		{
			name:          "xattr.conf",
			configName:    "xattr.conf",
			expectedError: `user etc file "xattr.conf" is managed by Talos`,
		},
		{
			name:          "cri prefix",
			configName:    "cri/conf.d/foo.part",
			expectedError: `user etc file "cri/conf.d/foo.part" is managed by Talos`,
		},
		{
			name:          "trusted roots",
			configName:    "ssl/certs/ca-certificates.crt",
			expectedError: `user etc file "ssl/certs/ca-certificates.crt" is managed by Talos`,
		},
		{
			name:          "cni prefix",
			configName:    "cni/net.d/10-flannel.conflist",
			expectedError: `user etc file "cni/net.d/10-flannel.conflist" is managed by Talos`,
		},
		{
			name:          "kubernetes prefix",
			configName:    "kubernetes/kubelet.conf",
			expectedError: `user etc file "kubernetes/kubelet.conf" is managed by Talos`,
		},
		{
			name:          "apparmor prefix",
			configName:    "apparmor/parser.conf",
			expectedError: `user etc file "apparmor/parser.conf" is managed by Talos`,
		},
		{
			name:          "apparmor.d prefix",
			configName:    "apparmor.d/profile",
			expectedError: `user etc file "apparmor.d/profile" is managed by Talos`,
		},
		{
			name:          "ca-certificates prefix",
			configName:    "ca-certificates/update.d/foo",
			expectedError: `user etc file "ca-certificates/update.d/foo" is managed by Talos`,
		},
		{
			name:          "ca-certificates exact",
			configName:    "ca-certificates",
			expectedError: `user etc file "ca-certificates" is managed by Talos`,
		},
		{
			name:          "lvm prefix",
			configName:    "lvm/lvm.conf",
			expectedError: `user etc file "lvm/lvm.conf" is managed by Talos`,
		},
		{
			name:          "pki prefix",
			configName:    "pki/tls/certs/ca-bundle.crt",
			expectedError: `user etc file "pki/tls/certs/ca-bundle.crt" is managed by Talos`,
		},
		{
			name:          "selinux prefix",
			configName:    "selinux/config",
			expectedError: `user etc file "selinux/config" is managed by Talos`,
		},
		{
			name:          "escaped path",
			configName:    "../foo",
			expectedError: `user etc file "../foo" must be a local path`,
		},
		{
			name:          "escaped nested path",
			configName:    "foo/../../bar",
			expectedError: `user etc file "foo/../../bar" must be a local path`,
		},
		{
			name:          "absolute path",
			configName:    "/foo",
			expectedError: `user etc file "/foo" must be a local path`,
		},
		{
			name:          "dot path",
			configName:    "./foo",
			expectedError: `user etc file "./foo" must be a local path`,
		},
		{
			name:          "dot prefix",
			configName:    ".foo",
			expectedError: `user etc file ".foo" must be a local path`,
		},
		{
			name:          "dot prefix nested",
			configName:    ".foo/bar",
			expectedError: `user etc file ".foo/bar" must be a local path`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := runtime.NewEtcFileConfigV1Alpha1(test.configName)

			warnings, err := cfg.Validate(validationMode{})
			assert.Empty(t, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
