// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain_test

import (
	"testing"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/domain"
)

func TestARM64PlatformHardware(t *testing.T) {
	inputs := testInputs(domain.ARM64{})

	definition, _, err := domain.Build(inputs)
	if err != nil {
		t.Fatal(err)
	}

	if definition.OS.Type.Arch != "aarch64" || len(definition.Clock.Timer) != 0 {
		t.Fatalf("arm64 platform: %#v %#v", definition.OS, definition.Clock)
	}

	if len(definition.Devices.Videos) != 0 || len(definition.Devices.Inputs) != 0 {
		t.Fatalf("arm64 video: %#v", definition.Devices)
	}

	if definition.Devices.Disks[0].Target.Bus != "scsi" {
		t.Fatalf("arm64 seed: %#v", definition.Devices.Disks[0])
	}

	if len(definition.Devices.Controllers) != 1 || definition.Devices.Controllers[0].Type != "scsi" || definition.Devices.Controllers[0].Model != "virtio-scsi" {
		t.Fatalf("arm64 controller: %#v", definition.Devices.Controllers)
	}
}
