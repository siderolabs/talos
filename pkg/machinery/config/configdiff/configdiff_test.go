// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configdiff_test

import (
	"net/url"
	"testing"

	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configdiff"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

func TestDiffString(t *testing.T) {
	t.Parallel()

	v1alpha1Cfg := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType:  "controlplane",
			MachineToken: "foo",
		},
	}

	v1alpha1CfgOther := v1alpha1Cfg.DeepCopy()
	v1alpha1CfgOther.MachineConfig.MachineType = "worker"

	siderolinkConfig := siderolink.NewConfigV1Alpha1()
	siderolinkConfig.APIUrlConfig = meta.URL{
		URL: must.Value(url.Parse("https://example.com"))(t),
	}

	for _, test := range []struct {
		name   string
		oldCfg []config.Document
		newCfg []config.Document
		want   string
	}{
		{
			name:   "empty",
			oldCfg: nil,
			newCfg: nil,
			want:   "",
		},
		{
			name:   "same",
			oldCfg: []config.Document{v1alpha1Cfg},
			newCfg: []config.Document{v1alpha1Cfg},
			want:   "",
		},
		{
			name:   "new doc",
			oldCfg: []config.Document{v1alpha1Cfg},
			newCfg: []config.Document{v1alpha1Cfg, siderolinkConfig},
			want:   "--- a\n+++ b\n@@ -4,3 +4,7 @@\n     token: foo\n     certSANs: []\n cluster: null\n+---\n+apiVersion: v1alpha1\n+kind: SideroLinkConfig\n+apiUrl: https://example.com\n",
		},
		{
			name:   "updated field",
			oldCfg: []config.Document{v1alpha1Cfg, siderolinkConfig},
			newCfg: []config.Document{v1alpha1CfgOther, siderolinkConfig},
			want:   "--- a\n+++ b\n@@ -1,6 +1,6 @@\n version: v1alpha1\n machine:\n-    type: controlplane\n+    type: worker\n     token: foo\n     certSANs: []\n cluster: null\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			oldCfg := must.Value(container.New(test.oldCfg...))(t)
			newCfg := must.Value(container.New(test.newCfg...))(t)

			got, err := configdiff.DiffToString(oldCfg, newCfg)
			require.NoError(t, err)

			require.Equal(t, test.want, got)
		})
	}
}
