// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package safepath_test

import (
	"testing"

	"github.com/talos-systems/talos/pkg/safepath"
)

func TestCleanPath(t *testing.T) {
	path := safepath.CleanPath("")
	if path != "" {
		t.Errorf("expected to receive empty string and received %s", path)
	}

	path = safepath.CleanPath("rootfs")
	if path != "rootfs" {
		t.Errorf("expected to receive 'rootfs' and received %s", path)
	}

	path = safepath.CleanPath("../../../var")
	if path != "var" {
		t.Errorf("expected to receive 'var' and received %s", path)
	}

	path = safepath.CleanPath("/../../../var")
	if path != "/var" {
		t.Errorf("expected to receive '/var' and received %s", path)
	}

	path = safepath.CleanPath("/foo/bar/")
	if path != "/foo/bar" {
		t.Errorf("expected to receive '/foo/bar' and received %s", path)
	}

	path = safepath.CleanPath("/foo/bar/../")
	if path != "/foo" {
		t.Errorf("expected to receive '/foo' and received %s", path)
	}
}
