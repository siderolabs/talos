// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sdboot_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/sdboot"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestGenerateAssets(t *testing.T) {
	t.Parallel()

	const (
		baseLoaderConf = "# systemd-boot configuration\n\ntimeout 10\n"
		ukiContents    = "fake-uki"
		sdBootContents = "fake-sd-boot"
		pkContents     = "fake-pk-auth"
		kekContents    = "fake-kek-auth"
		dbContents     = "fake-db-auth"
		ukiFileName    = "Talos-1.13.0.efi"
	)

	for _, tc := range []struct {
		name           string
		enrollKeys     string
		expectedConf   string
		expectKeysAuto bool
	}{
		{
			name:           "no_enrollment",
			enrollKeys:     "",
			expectedConf:   baseLoaderConf,
			expectKeysAuto: false,
		},
		{
			name:           "if_safe",
			enrollKeys:     "if-safe",
			expectedConf:   baseLoaderConf + "\nsecure-boot-enroll if-safe\n",
			expectKeysAuto: true,
		},
		{
			name:           "force",
			enrollKeys:     "force",
			expectedConf:   baseLoaderConf + "\nsecure-boot-enroll force\n",
			expectKeysAuto: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mountPrefix := t.TempDir()
			srcDir := t.TempDir()

			ukiPath := filepath.Join(srcDir, "uki.efi")
			sdBootPath := filepath.Join(srcDir, "systemd-bootx64.efi")
			pkPath := filepath.Join(srcDir, "PK.auth")
			kekPath := filepath.Join(srcDir, "KEK.auth")
			dbPath := filepath.Join(srcDir, "db.auth")

			require.NoError(t, os.WriteFile(ukiPath, []byte(ukiContents), 0o644))
			require.NoError(t, os.WriteFile(sdBootPath, []byte(sdBootContents), 0o644))
			require.NoError(t, os.WriteFile(pkPath, []byte(pkContents), 0o644))
			require.NoError(t, os.WriteFile(kekPath, []byte(kekContents), 0o644))
			require.NoError(t, os.WriteFile(dbPath, []byte(dbContents), 0o644))

			opts := options.InstallOptions{
				Arch:        "amd64",
				MountPrefix: mountPrefix,
				BootAssets: options.BootAssets{
					UKIPath:    ukiPath,
					SDBootPath: sdBootPath,
				},
				SecureBootEnrollKeys: tc.enrollKeys,
				PlatformKeyPath:      pkPath,
				KeyExchangeKeyPath:   kekPath,
				SignatureKeyPath:     dbPath,
				Printf:               func(string, ...any) {},
			}

			require.NoError(t, sdboot.GenerateAssets(&sdboot.Config{}, opts, ukiFileName))

			loaderDir := filepath.Join(mountPrefix, constants.EFIMountPoint, "loader")

			confBytes, err := os.ReadFile(filepath.Join(loaderDir, "loader.conf"))
			require.NoError(t, err)
			require.Equal(t, tc.expectedConf, string(confBytes))

			keysDir := filepath.Join(loaderDir, "keys", "auto")

			if tc.expectKeysAuto {
				for filename, expectedContent := range map[string]string{
					constants.PlatformKeyAsset:    pkContents,
					constants.KeyExchangeKeyAsset: kekContents,
					constants.SignatureKeyAsset:   dbContents,
				} {
					got, err := os.ReadFile(filepath.Join(keysDir, filename))
					require.NoError(t, err, "reading %s", filename)
					require.Equal(t, expectedContent, string(got), "content of %s", filename)
				}
			} else {
				_, err := os.Stat(keysDir)
				require.True(t, os.IsNotExist(err), "loader/keys/auto should not exist when no enrollment keys configured")
			}

			// UKI is copied to EFI/Linux/<ukiFileName>
			gotUKI, err := os.ReadFile(filepath.Join(mountPrefix, constants.EFIMountPoint, "EFI", "Linux", ukiFileName))
			require.NoError(t, err)
			require.Equal(t, ukiContents, string(gotUKI))

			// sd-boot is copied to EFI/boot/BOOTX64.efi (amd64)
			gotSDBoot, err := os.ReadFile(filepath.Join(mountPrefix, constants.EFIMountPoint, "EFI", "boot", "BOOTX64.efi"))
			require.NoError(t, err)
			require.Equal(t, sdBootContents, string(gotSDBoot))
		})
	}
}
