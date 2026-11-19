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
