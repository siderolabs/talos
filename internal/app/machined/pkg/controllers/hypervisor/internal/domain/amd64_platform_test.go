// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain_test

import (
	"testing"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/domain"
)

func TestAMD64PlatformHardware(t *testing.T) {
	inputs := testInputs(domain.AMD64{})

	definition, _, err := domain.Build(inputs)
	if err != nil {
		t.Fatal(err)
	}

	if definition.OS.Type.Arch != "x86_64" || len(definition.Clock.Timer) != 3 {
		t.Fatalf("amd64 platform: %#v %#v", definition.OS, definition.Clock)
	}

	if len(definition.Devices.Videos) != 1 || definition.Devices.Videos[0].Model.Type != "vga" || len(definition.Devices.Inputs) != 2 {
		t.Fatalf("amd64 video: %#v", definition.Devices)
	}

	if definition.Devices.Disks[0].Target.Bus != "sata" {
		t.Fatalf("amd64 seed: %#v", definition.Devices.Disks[0])
	}

	for _, controller := range definition.Devices.Controllers {
		if controller.Type == "scsi" {
			t.Fatal("unexpected explicit amd64 SCSI controller")
		}
	}
}

func TestAMD64VideoWithoutVNC(t *testing.T) {
	for _, tt := range []struct {
		name        string
		vnc, video  bool
		videos, gfx int
	}{
		{"video only", false, true, 1, 0},
		{"neither", false, false, 0, 0},
		{"VNC", true, true, 1, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			inputs := testInputs(domain.AMD64{})
			inputs.Spec.Console.VNC = tt.vnc
			inputs.Spec.Video.Enabled = tt.video

			definition, _, err := domain.Build(inputs)
			if err != nil {
				t.Fatal(err)
			}

			if len(definition.Devices.Videos) != tt.videos || len(definition.Devices.Graphics) != tt.gfx {
				t.Fatalf("videos %d graphics %d, want %d %d", len(definition.Devices.Videos), len(definition.Devices.Graphics), tt.videos, tt.gfx)
			}
		})
	}
}
