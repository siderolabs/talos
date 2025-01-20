// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package platforms

import (
	"github.com/blang/semver/v4"

	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

// SBC describes a Single Board Computer configuration.
type SBC struct {
	Name string

	// For Talos < 1.7.
	BoardName string

	// For Talos 1.7+
	OverlayName  string
	OverlayImage string

	Label         string
	Documentation string

	MinVersion semver.Version
}

// DiskImagePath returns the path to the disk image for the SBC.
func (s SBC) DiskImagePath(talosVersion string) string {
	if quirks.New(talosVersion).SupportsOverlay() {
		return "metal-arm64.raw.xz"
	}

	return "metal-" + s.BoardName + "-arm64.raw.xz"
}

// SBCs returns a list of supported Single Board Computers.
func SBCs() []SBC {
	return []SBC{
		{
			Name: "rpi_generic",

			BoardName: "rpi_generic",

			OverlayName:  "rpi_generic",
			OverlayImage: "siderolabs/sbc-raspberrypi",

			Label:         "Raspberry Pi Series",
			Documentation: "/talos-guides/install/single-board-computers/rpi_generic/",
		},
		{
			Name: "bananapi_m64",

			BoardName: "bananapi_m64",

			OverlayName:  "bananapi_m64",
			OverlayImage: "siderolabs/sbc-allwinner",

			Label:         "Banana Pi M64",
			Documentation: "/talos-guides/install/single-board-computers/bananapi_m64/",
		},
		{
			Name: "nanopi_r4s",

			BoardName: "rockpi_4",

			OverlayName:  "nanopi-r4s",
			OverlayImage: "siderolabs/sbc-rockchip",

			Label:         "Friendlyelec Nano PI R4S",
			Documentation: "/talos-guides/install/single-board-computers/nanopi_r4s/",

			MinVersion: semver.MustParse("1.3.0"),
		},
		{
			Name: "nanopi_r5s",

			OverlayName:  "nanopi-r5s",
			OverlayImage: "siderolabs/sbc-rockchip",

			Label: "Friendlyelec Nano PI R5S",

			MinVersion: semver.MustParse("1.8.0-alpha.2"),
		},
		{
			Name: "jetson_nano",

			BoardName: "jetson_nano",

			OverlayName:  "jetson_nano",
			OverlayImage: "siderolabs/sbc-jetson",

			Label:         "Jetson Nano",
			Documentation: "/talos-guides/install/single-board-computers/jetson_nano/",
		},
		{
			Name: "libretech_all_h3_cc_h5",

			BoardName: "libretech_all_h3_cc_h5",

			OverlayName:  "libretech_all_h3_cc_h5",
			OverlayImage: "siderolabs/sbc-allwinner",

			Label:         "Libre Computer Board ALL-H3-CC",
			Documentation: "/talos-guides/install/single-board-computers/libretech_all_h3_cc_h5/",
		},
		{
			Name: "orangepi_r1_plus_lts",

			OverlayName:  "orangepi-r1-plus-lts",
			OverlayImage: "siderolabs/sbc-rockchip",

			Label:         "Orange Pi R1 Plus LTS",
			Documentation: "/talos-guides/install/single-board-computers/orangepi_r1_plus_lts/",

			MinVersion: semver.MustParse("1.7.0"),
		},
		{
			Name: "pine64",

			BoardName: "pine64",

			OverlayName:  "pine64",
			OverlayImage: "siderolabs/sbc-allwinner",

			Label:         "Pine64",
			Documentation: "/talos-guides/install/single-board-computers/pine64/",
		},
		{
			Name: "rock64",

			BoardName: "rock64",

			OverlayName:  "rock64",
			OverlayImage: "siderolabs/sbc-rockchip",

			Label:         "Pine64 Rock64",
			Documentation: "/talos-guides/install/single-board-computers/rock64/",
		},
		{
			Name: "rock4cplus",

			OverlayName:  "rock4cplus",
			OverlayImage: "siderolabs/sbc-rockchip",

			Label:         "Radxa ROCK 4C Plus",
			Documentation: "/talos-guides/install/single-board-computers/rock4cplus/",

			MinVersion: semver.MustParse("1.7.0"),
		},
		{
			Name: "rock4se",

			OverlayName:  "rock4se",
			OverlayImage: "siderolabs/sbc-rockchip",

			Label:         "Radxa ROCK 4SE",
			Documentation: "", // missing

			MinVersion: semver.MustParse("1.8.0-alpha.1"),
		},
		{
			Name: "rock5b",

			OverlayName:  "rock5b",
			OverlayImage: "siderolabs/sbc-rockchip",

			Label:         "Radxa ROCK 5B",
			Documentation: "/talos-guides/install/single-board-computers/rock5b/",

			MinVersion: semver.MustParse("1.9.2"),
		},
		{
			Name: "rockpi_4",

			BoardName: "rockpi_4",

			OverlayName:  "rockpi4",
			OverlayImage: "siderolabs/sbc-rockchip",

			Label:         "Radxa ROCK PI 4",
			Documentation: "/talos-guides/install/single-board-computers/rockpi_4/",
		},
		{
			Name: "rockpi_4c",

			BoardName: "rockpi_4c",

			OverlayName:  "rockpi4c",
			OverlayImage: "siderolabs/sbc-rockchip",

			Label:         "Radxa ROCK PI 4C",
			Documentation: "/talos-guides/install/single-board-computers/rockpi_4c/",
		},
		{
			Name: "helios64",

			OverlayName:  "helios64",
			OverlayImage: "siderolabs/sbc-rockchip",

			Label:         "Kobol Helios64",
			Documentation: "", // missing

			MinVersion: semver.MustParse("1.8.0-alpha.2"),
		},
		{
			Name: "turingrk1",

			OverlayName:  "turingrk1",
			OverlayImage: "siderolabs/sbc-rockchip",

			Label:         "Turing RK1",
			Documentation: "/talos-guides/install/single-board-computers/turing_rk1/",

			MinVersion: semver.MustParse("1.9.0-beta.0"),
		},
	}
}
