// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/pkg/cluster/kubernetes"
)

func TestPath(t *testing.T) {
	t.Parallel()

	t.Run("Valid", func(t *testing.T) {
		t.Parallel()

		options := &kubernetes.UpgradeOptions{
			FromVersion: "1.20.5",
			ToVersion:   "1.21.0-beta.0",
		}

		assert.Equal(t, "1.20->1.21", options.Path())
	})

	t.Run("Invalid", func(t *testing.T) {
		t.Parallel()

		options := &kubernetes.UpgradeOptions{
			FromVersion: "foo",
			ToVersion:   "bar",
		}

		assert.Equal(t, "", options.Path())
	})
}
