// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pkg

import (
	"fmt"
	"strings"

	"github.com/talos-systems/go-cmd/pkg/cmd"
)

// Loattach attaches a loopback device by inoking the `losetup` command.
func Loattach(img string) (dev string, err error) {
	if dev, err = cmd.Run("losetup", "--find", "--partscan", "--nooverlap", "--show", img); err != nil {
		return "", fmt.Errorf("failed to setup loopback device: %w", err)
	}

	return strings.TrimSuffix(dev, "\n"), nil
}

// Lodetach detaches a loopback device by inoking the `losetup` command.
func Lodetach(img string) (err error) {
	if _, err = cmd.Run("losetup", "-d", img); err != nil {
		return fmt.Errorf("failed to detach loopback device: %w", err)
	}

	return nil
}
