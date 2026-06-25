// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package rotatepatcher provides a config patcher which modifies machine configuration to rotate PKI.
package rotatepatcher

import "github.com/siderolabs/talos/pkg/machinery/config"

func hasDocument(kind string, cfg config.Container) bool {
	for _, doc := range cfg.Documents() {
		if doc.Kind() == kind {
			return true
		}
	}

	return false
}
