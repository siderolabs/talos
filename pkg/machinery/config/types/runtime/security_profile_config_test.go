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

//go:embed testdata/securityprofileconfig.yaml
var expectedSecurityProfileConfigDocument []byte

func TestSecurityProfileConfigMarshalStability(t *testing.T) {
	cfg := runtime.NewSecurityProfileConfigV1Alpha1()
	cfg.WorkloadIsolationEnabled = new(true)

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedSecurityProfileConfigDocument, marshaled)
}

func TestSecurityProfileConfigWorkloadIsolation(t *testing.T) {
	t.Parallel()

	// absent field -> disabled
	assert.False(t, runtime.NewSecurityProfileConfigV1Alpha1().WorkloadIsolation())

	cfg := runtime.NewSecurityProfileConfigV1Alpha1()
	cfg.WorkloadIsolationEnabled = new(true)
	assert.True(t, cfg.WorkloadIsolation())

	cfg.WorkloadIsolationEnabled = new(false)
	assert.False(t, cfg.WorkloadIsolation())

	warnings, err := cfg.Validate(validationMode{})
	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestSecurityProfileConfigIgnoreCtrlAltDelete(t *testing.T) {
	t.Parallel()

	// absent field -> reboot on Ctrl-Alt-Delete
	assert.False(t, runtime.NewSecurityProfileConfigV1Alpha1().IgnoreCtrlAltDelete())

	cfg := runtime.NewSecurityProfileConfigV1Alpha1()
	cfg.IgnoreCtrlAltDeleteEnabled = new(true)
	assert.True(t, cfg.IgnoreCtrlAltDelete())

	cfg.IgnoreCtrlAltDeleteEnabled = new(false)
	assert.False(t, cfg.IgnoreCtrlAltDelete())
}

func TestSecurityProfileConfigSELinuxCategories(t *testing.T) {
	t.Parallel()

	// absent field -> workloads run without categories
	assert.False(t, runtime.NewSecurityProfileConfigV1Alpha1().SELinuxCategories())

	cfg := runtime.NewSecurityProfileConfigV1Alpha1()
	cfg.SELinuxCategoriesEnabled = new(true)
	assert.True(t, cfg.SELinuxCategories())

	cfg.SELinuxCategoriesEnabled = new(false)
	assert.False(t, cfg.SELinuxCategories())
}

func TestSecurityProfileConfigSELinuxBooleans(t *testing.T) {
	t.Parallel()

	assert.Empty(t, runtime.NewSecurityProfileConfigV1Alpha1().SELinuxBooleans())

	cfg := runtime.NewSecurityProfileConfigV1Alpha1()
	cfg.SELinuxBooleansConfig = map[string]bool{"pod_write_cni_plugins": false, "ext_iscsid_clients": true}
	assert.Equal(t, map[string]bool{"pod_write_cni_plugins": false, "ext_iscsid_clients": true}, cfg.SELinuxBooleans())

	_, err := cfg.Validate(validationMode{})
	require.NoError(t, err)

	// the static pods of the control plane run in their own domains with categories, so pod_t no longer uses their files
	cfg.SELinuxCategoriesEnabled = new(true)
	assert.Equal(t, map[string]bool{"pod_write_cni_plugins": false, "ext_iscsid_clients": true, "pod_use_control_plane_files": false}, cfg.SELinuxBooleans())

	cfg.SELinuxBooleansConfig = map[string]bool{"pod_use_control_plane_files": true}
	assert.Equal(t, map[string]bool{"pod_use_control_plane_files": true}, cfg.SELinuxBooleans())

	cfg.SELinuxBooleansConfig = map[string]bool{"Pod-Use-NFS": true, "pod_use_ebpf": false}
	_, err = cfg.Validate(validationMode{})
	require.EqualError(t, err, `invalid SELinux boolean name "Pod-Use-NFS"`)
}
