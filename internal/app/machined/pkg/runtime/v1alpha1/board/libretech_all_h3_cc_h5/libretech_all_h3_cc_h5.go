// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package libretechallh3cch5

import (
	"fmt"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// LibretechAllH3CCH5 represents the Libre Computer ALL-H3-CC (Tritium).
//
// Reference: https://libre.computer/products/boards/all-h3-cc/
type LibretechAllH3CCH5 struct{}

// Name implenents the runtime.Board.
func (l *LibretechAllH3CCH5) Name() string {
	return constants.BoardLibretechAllH3CCH5
}

// UBoot implenents the runtime.Board.
func (l *LibretechAllH3CCH5) UBoot() (string, int64) {
	return fmt.Sprintf("/usr/install/u-boot/%s/u-boot-sunxi-with-spl.bin", constants.BoardLibretechAllH3CCH5), 1024 * 8
}

// PartitionOptions implenents the runtime.Board.
func (l *LibretechAllH3CCH5) PartitionOptions() *runtime.PartitionOptions {
	return &runtime.PartitionOptions{PartitionsOffset: 2048}
}
