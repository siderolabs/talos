// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package configdiff provides a way to compare two config trees.
package configdiff

import (
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/textdiff"
)

// DiffConfigs returns a string representation of the diff between two machine configurations.
//
// One of the resources might be nil.
func DiffConfigs(oldCfg, newCfg config.Encoder) (string, error) {
	var (
		oldYaml, newYaml []byte
		err              error
	)

	if oldCfg != nil {
		oldYaml, err = oldCfg.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
		if err != nil {
			return "", err
		}
	}

	if newCfg != nil {
		newYaml, err = newCfg.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
		if err != nil {
			return "", err
		}
	}

	return textdiff.Diff(string(oldYaml), string(newYaml))
}
