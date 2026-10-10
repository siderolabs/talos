// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/freddierice/go-losetup/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestTemporaryGrowMountProjectQuota(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("test requires root privileges")
	}

	if hostname, _ := os.Hostname(); hostname == "buildkitsandbox" { //nolint:errcheck
		t.Skip("test not supported under buildkit as loop devices are not propagated from /dev")
	}

	for _, enabled := range []bool{false, true} {
		name := "without project quotas"
		if enabled {
			name = "with project quotas"
		}

		t.Run(name, func(t *testing.T) {
			image, err := os.Create(filepath.Join(t.TempDir(), "xfs.raw"))
			require.NoError(t, err)
			require.NoError(t, image.Truncate(512<<20))
			require.NoError(t, image.Close())

			device, err := losetup.Attach(image.Name(), 0, false)
			require.NoError(t, err)
			t.Cleanup(func() {
				assert.NoError(t, device.Detach())
			})

			output, err := exec.CommandContext(t.Context(), "mkfs.xfs", "-f", device.Path()).CombinedOutput()
			require.NoError(t, err, "%s", output)

			cfg := block.NewVolumeConfig(block.NamespaceName, "TEST")
			cfg.TypedSpec().Provisioning.FilesystemSpec.Type = block.FilesystemTypeXFS
			cfg.TypedSpec().Mount.ProjectQuotaSupport = enabled
			volumeContext := ManagerContext{
				Cfg: cfg,
				Status: &block.VolumeStatusSpec{
					MountLocation: device.Path(),
				},
			}

			// Repeat the grow mount: subsequent boots must not turn accounting
			// off after it was enabled by the previous boot's final mount.
			for range 2 {
				require.NoError(t, withTemporaryMount(zaptest.NewLogger(t), volumeContext, func(target string) error {
					mounts, readErr := os.ReadFile("/proc/self/mounts")
					require.NoError(t, readErr)

					for line := range strings.SplitSeq(string(mounts), "\n") {
						fields := strings.Fields(line)
						if len(fields) < 4 || fields[1] != target {
							continue
						}

						assert.Equal(t, "xfs", fields[2])
						assert.Equal(t, enabled, strings.Contains(","+fields[3]+",", ",prjquota,"))

						return nil
					}

					t.Errorf("temporary grow mount %q not found", target)

					return nil
				}))
			}
		})
	}
}
