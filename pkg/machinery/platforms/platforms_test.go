// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package platforms_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/platforms"
)

func TestPlatform(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		platform platforms.Platform

		expectedDefaultDiskImagePath string
		expectedISOPath              string
		expectedSecureBootISOPath    string
		expectedPXEPath              string
		expectedSecureBootPXEPath    string
		expectedUKIPath              string
		expectedSecureBootUKIPath    string
		expectedKernelPath           string
		expectedInitramfsPath        string
		expectedCmdlinePath          string
		expectedNotOnlyDiskImage     bool
	}{
		{
			name: "metal",

			platform: platforms.MetalPlatform(),

			expectedDefaultDiskImagePath: "metal-amd64.raw.zst",
			expectedISOPath:              "metal-amd64.iso",
			expectedSecureBootISOPath:    "metal-amd64-secureboot.iso",
			expectedPXEPath:              "metal-amd64",
			expectedSecureBootPXEPath:    "metal-amd64-secureboot",
			expectedUKIPath:              "metal-amd64-uki.efi",
			expectedSecureBootUKIPath:    "metal-amd64-secureboot-uki.efi",
			expectedKernelPath:           "kernel-amd64",
			expectedInitramfsPath:        "initramfs-amd64.xz",
			expectedCmdlinePath:          "cmdline-metal-amd64",
			expectedNotOnlyDiskImage:     true,
		},
		{
			name: "aws",

			platform: platforms.CloudPlatforms()[0],

			expectedDefaultDiskImagePath: "aws-amd64.raw.xz",
			expectedISOPath:              "aws-amd64.iso",
			expectedSecureBootISOPath:    "aws-amd64-secureboot.iso",
			expectedPXEPath:              "aws-amd64",
			expectedSecureBootPXEPath:    "aws-amd64-secureboot",
			expectedUKIPath:              "aws-amd64-uki.efi",
			expectedSecureBootUKIPath:    "aws-amd64-secureboot-uki.efi",
			expectedKernelPath:           "kernel-amd64",
			expectedInitramfsPath:        "initramfs-amd64.xz",
			expectedCmdlinePath:          "cmdline-aws-amd64",
			expectedNotOnlyDiskImage:     false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expectedDefaultDiskImagePath, test.platform.DiskImageDefaultPath(platforms.ArchAmd64))
			assert.Equal(t, test.expectedISOPath, test.platform.ISOPath(platforms.ArchAmd64))
			assert.Equal(t, test.expectedSecureBootISOPath, test.platform.SecureBootISOPath(platforms.ArchAmd64))
			assert.Equal(t, test.expectedPXEPath, test.platform.PXEScriptPath(platforms.ArchAmd64))
			assert.Equal(t, test.expectedSecureBootPXEPath, test.platform.SecureBootPXEScriptPath(platforms.ArchAmd64))
			assert.Equal(t, test.expectedUKIPath, test.platform.UKIPath(platforms.ArchAmd64))
			assert.Equal(t, test.expectedSecureBootUKIPath, test.platform.SecureBootUKIPath(platforms.ArchAmd64))
			assert.Equal(t, test.expectedKernelPath, test.platform.KernelPath(platforms.ArchAmd64))
			assert.Equal(t, test.expectedInitramfsPath, test.platform.InitramfsPath(platforms.ArchAmd64))
			assert.Equal(t, test.expectedCmdlinePath, test.platform.CmdlinePath(platforms.ArchAmd64))
			assert.Equal(t, test.expectedNotOnlyDiskImage, test.platform.NotOnlyDiskImage())
		})
	}
}
