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
	var repairFunc func(partition string) error

	switch p.fstype {
	case "ext4":
		repairFunc = makefs.Ext4Repair
	case "xfs":
		repairFunc = makefs.XFSRepair
	default:
		return fmt.Errorf("unsupported filesystem type for repair: %s", p.fstype)
	}

	printerOptions.Printf("filesystem (%s) on %s needs cleaning, running repair", p.fstype, p.source)

	if err := repairFunc(p.source); err != nil {
		return fmt.Errorf("repair: %w", err)
	}

	printerOptions.Printf("filesystem successfully repaired on %s", p.source)

	return nil
}
