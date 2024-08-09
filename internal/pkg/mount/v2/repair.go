// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"fmt"

	"github.com/siderolabs/talos/pkg/makefs"
)

// repair a filesystem.
func (p *Point) repair(printerOptions PrinterOptions) error {
	printerOptions.Printf("filesystem on %s needs cleaning, running repair", p.source)

	if err := makefs.XFSRepair(p.source, p.fstype); err != nil {
		return fmt.Errorf("xfs_repair: %w", err)
	}

	printerOptions.Printf("filesystem successfully repaired on %s", p.source)

	return nil
}
