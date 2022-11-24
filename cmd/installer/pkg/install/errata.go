// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"log"
	"os"

	pkgkernel "github.com/siderolabs/talos/pkg/kernel"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
)

// errataBTF handles the case when kexec from pre-BTF kernel to BTF enabled kernel always fails.
//
// This applies to upgrades of Talos < 1.3.0 to Talos >= 1.3.0.
func errataBTF() {
	_, err := os.Stat("/sys/kernel/btf/vmlinux")
	if err == nil {
		// BTF is enabled, nothing to do
		return
	}

	log.Printf("disabling kexec due to upgrade to the BTF enabled kernel")

	if err = pkgkernel.WriteParam(&kernel.Param{
		Key:   "proc.sys.kernel.kexec_load_disabled",
		Value: "1",
	}); err != nil {
		log.Printf("failed to disable kexec: %s", err)
	}
}
