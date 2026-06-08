// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package global provides global flags for talosctl.
package global

import (
	"errors"
)

// ErrConfigContext is returned when config context cannot be resolved.
var ErrConfigContext = errors.New("failed to resolve config context")

// Args is a context for the Talos command line client.
type Args struct {
	Talosconfig     string
	CmdContext      string
	Cluster         string
	Nodes           []string
	Endpoints       []string
	SideroV1KeysDir string
}
