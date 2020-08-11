// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/talos-systems/talos/pkg/conditions"
)

type writerReporter struct {
	w        io.Writer
	lastLine string
}

func (wr *writerReporter) Update(condition conditions.Condition) {
	line := fmt.Sprintf("waiting for %s", condition)

	if line != wr.lastLine {
		fmt.Fprintln(wr.w, strings.TrimSpace(line))
		wr.lastLine = line
	}
}

// StderrReporter returns console reporter with stderr output.
func StderrReporter() Reporter {
	return &writerReporter{
		w: os.Stderr,
	}
}
