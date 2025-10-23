// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

func createOutputWithDefaults(kind profile.OutputKind, arch, version string, secureBoot bool) profile.Output {
	out := profile.Output{
		Kind: kind,
	}

	out.FillDefaults(arch, version, secureBoot)

	return out
}

func createOutputWithOverride(kind profile.OutputKind, bootloader profile.BootloaderKind, arch, version string, secureBoot bool) profile.Output {
	out := profile.Output{
		Kind: kind,
	}

	switch kind { //nolint:exhaustive
	case profile.OutKindImage:
		out.ImageOptions = &profile.ImageOptions{
			Bootloader: bootloader,
		}
	case profile.OutKindISO:
		if quirks.New(version).ISOSupportsSettingBootloader() {
			out.ISOOptions = &profile.ISOOptions{
				Bootloader: bootloader,
			}
		}
	}

	out.FillDefaults(arch, version, secureBoot)

	return out
}

func TestBootloaderSetting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		arch       string
		version    string
		secureBoot bool
		wantImage  profile.BootloaderKind
	}{
		// Talos < 1.10: GRUB for both amd64/arm64, ISO options not supported
		{"amd64", "1.9.0", false, profile.BootLoaderKindGrub},
		{"amd64", "1.9.0", true, profile.BootLoaderKindSDBoot},
		{"arm64", "1.9.0", false, profile.BootLoaderKindGrub},
		{"arm64", "1.9.0", true, profile.BootLoaderKindSDBoot},

		// Talos 1.10-1.11: amd64=dual-boot, arm64=sd-boot, ISO options not supported
		{"amd64", "1.10.0", false, profile.BootLoaderKindDualBoot},
		{"amd64", "1.10.0", true, profile.BootLoaderKindSDBoot},
		{"arm64", "1.10.0", false, profile.BootLoaderKindSDBoot},
		{"arm64", "1.10.0", true, profile.BootLoaderKindSDBoot},
		{"amd64", "1.11.0", false, profile.BootLoaderKindDualBoot},
		{"amd64", "1.11.0", true, profile.BootLoaderKindSDBoot},
		{"arm64", "1.11.0", false, profile.BootLoaderKindSDBoot},
		{"arm64", "1.11.0", true, profile.BootLoaderKindSDBoot},

		// Talos >= 1.12: amd64=dual-boot, arm64=sd-boot, ISO options supported
		{"amd64", "1.12.0", false, profile.BootLoaderKindDualBoot},
		{"amd64", "1.12.0", true, profile.BootLoaderKindSDBoot},
		{"arm64", "1.12.0", false, profile.BootLoaderKindSDBoot},
		{"arm64", "1.12.0", true, profile.BootLoaderKindSDBoot},
	}

	for _, tt := range tests {
		name := tt.arch + "-" + tt.version
		if tt.secureBoot {
			name += "-secureboot"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Test Image output
			img := createOutputWithDefaults(profile.OutKindImage, tt.arch, tt.version, tt.secureBoot)
			require.NotNil(t, img.ImageOptions)
			require.Equal(t, tt.wantImage, img.ImageOptions.Bootloader)

			// Test ISO output
			iso := createOutputWithDefaults(profile.OutKindISO, tt.arch, tt.version, tt.secureBoot)
			if quirks.New(tt.version).ISOSupportsSettingBootloader() {
				require.NotNil(t, iso.ISOOptions)
				require.Equal(t, tt.wantImage, iso.ISOOptions.Bootloader)
			} else {
				require.Nil(t, iso.ISOOptions)
			}
		})
	}
}

func TestBootloaderOverride(t *testing.T) {
	t.Parallel()

	tests := []struct {
		arch       string
		version    string
		secureBoot bool
		override   profile.BootloaderKind
		wantImage  profile.BootloaderKind
	}{
		// Talos < 1.10: GRUB is forced, overrides are ignored
		{"amd64", "1.9.0", false, profile.BootLoaderKindGrub, profile.BootLoaderKindGrub},
		{"amd64", "1.9.0", false, profile.BootLoaderKindSDBoot, profile.BootLoaderKindGrub},   // forced to GRUB
		{"amd64", "1.9.0", false, profile.BootLoaderKindDualBoot, profile.BootLoaderKindGrub}, // forced to GRUB
		{"amd64", "1.9.0", true, profile.BootLoaderKindGrub, profile.BootLoaderKindSDBoot},    // secureboot forces sd-boot
		{"arm64", "1.9.0", false, profile.BootLoaderKindGrub, profile.BootLoaderKindGrub},
		{"arm64", "1.9.0", false, profile.BootLoaderKindSDBoot, profile.BootLoaderKindGrub}, // forced to GRUB
		{"arm64", "1.9.0", true, profile.BootLoaderKindGrub, profile.BootLoaderKindSDBoot},  // secureboot forces sd-boot

		// Talos 1.10-1.11: amd64 respects override, arm64 forced to sd-boot
		{"amd64", "1.10.0", false, profile.BootLoaderKindGrub, profile.BootLoaderKindGrub},
		{"amd64", "1.10.0", false, profile.BootLoaderKindSDBoot, profile.BootLoaderKindSDBoot},
		{"amd64", "1.10.0", false, profile.BootLoaderKindDualBoot, profile.BootLoaderKindDualBoot},
		{"amd64", "1.10.0", true, profile.BootLoaderKindGrub, profile.BootLoaderKindSDBoot},  // secureboot forces sd-boot
		{"arm64", "1.10.0", false, profile.BootLoaderKindGrub, profile.BootLoaderKindSDBoot}, // arm64 >= 1.10 forces sd-boot
		{"arm64", "1.10.0", true, profile.BootLoaderKindGrub, profile.BootLoaderKindSDBoot},  // secureboot forces sd-boot
		{"amd64", "1.11.0", false, profile.BootLoaderKindGrub, profile.BootLoaderKindGrub},
		{"amd64", "1.11.0", false, profile.BootLoaderKindDualBoot, profile.BootLoaderKindDualBoot},
		{"amd64", "1.11.0", true, profile.BootLoaderKindGrub, profile.BootLoaderKindSDBoot},  // secureboot forces sd-boot
		{"arm64", "1.11.0", false, profile.BootLoaderKindGrub, profile.BootLoaderKindSDBoot}, // arm64 >= 1.10 forces sd-boot
		{"arm64", "1.11.0", true, profile.BootLoaderKindGrub, profile.BootLoaderKindSDBoot},  // secureboot forces sd-boot

		// Talos >= 1.12: amd64 respects override, arm64 forced to sd-boot
		{"amd64", "1.12.0", false, profile.BootLoaderKindGrub, profile.BootLoaderKindGrub},
		{"amd64", "1.12.0", false, profile.BootLoaderKindSDBoot, profile.BootLoaderKindSDBoot},
		{"amd64", "1.12.0", false, profile.BootLoaderKindDualBoot, profile.BootLoaderKindDualBoot},
		{"amd64", "1.12.0", true, profile.BootLoaderKindGrub, profile.BootLoaderKindSDBoot},  // secureboot forces sd-boot
		{"arm64", "1.12.0", false, profile.BootLoaderKindGrub, profile.BootLoaderKindSDBoot}, // arm64 >= 1.10 forces sd-boot
		{"arm64", "1.12.0", true, profile.BootLoaderKindGrub, profile.BootLoaderKindSDBoot},  // secureboot forces sd-boot
	}

	for _, tt := range tests {
		name := tt.arch + "-" + tt.version + "-override-" + tt.override.String()
		if tt.secureBoot {
			name += "-secureboot"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Test Image output with override
			img := createOutputWithOverride(profile.OutKindImage, tt.override, tt.arch, tt.version, tt.secureBoot)
			require.NotNil(t, img.ImageOptions)
			require.Equal(t, tt.wantImage, img.ImageOptions.Bootloader)

			// Test ISO output with override
			iso := createOutputWithOverride(profile.OutKindISO, tt.override, tt.arch, tt.version, tt.secureBoot)
			if quirks.New(tt.version).ISOSupportsSettingBootloader() {
				require.NotNil(t, iso.ISOOptions)
				require.Equal(t, tt.wantImage, iso.ISOOptions.Bootloader)
			} else {
				require.Nil(t, iso.ISOOptions)
			}
		})
	}
}
