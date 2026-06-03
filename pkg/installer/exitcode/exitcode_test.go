// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package exitcode_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/assert"

	installpkg "github.com/siderolabs/talos/cmd/installer/pkg/install"
	pkgimager "github.com/siderolabs/talos/pkg/imager"
	profilepkg "github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/installer/exitcode"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, constants.ExitSuccess, exitcode.Resolve(nil))
	})

	t.Run("unknown", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, constants.ExitUnknownError, exitcode.Resolve(errors.New("boom")))
	})

	for _, test := range []struct {
		name string
		err  error
		code int
	}{
		{name: "invalid-input", err: xerrors.NewTagged[profilepkg.InvalidInputTag](errors.New("bad input")), code: constants.ExitInvalidInput},
		{name: "unsupported", err: xerrors.NewTagged[profilepkg.UnsupportedTag](errors.New("unsupported")), code: constants.ExitUnsupported},
		{name: "environment", err: xerrors.NewTagged[installpkg.EnvironmentTag](errors.New("env")), code: constants.ExitEnvironment},
		{name: "dependency", err: xerrors.NewTagged[pkgimager.DependencyTag](errors.New("dep")), code: constants.ExitDependency},
		{name: "io", err: xerrors.NewTagged[pkgimager.IOTag](errors.New("io")), code: constants.ExitIO},
		{name: "install", err: xerrors.NewTagged[installpkg.InstallTag](errors.New("install")), code: constants.ExitInstall},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.code, exitcode.Resolve(test.err))
			assert.Equal(t, test.code, exitcode.Resolve(fmt.Errorf("wrapped: %w", test.err)))
		})
	}
}

func TestResolvePriority(t *testing.T) {
	t.Parallel()

	err := xerrors.NewTagged[installpkg.InstallTag](
		xerrors.NewTagged[pkgimager.DependencyTag](
			xerrors.NewTagged[profilepkg.InvalidInputTag](errors.New("bad")),
		),
	)

	assert.Equal(t, constants.ExitInvalidInput, exitcode.Resolve(err))
}

func TestTagged(t *testing.T) {
	t.Parallel()

	assert.False(t, exitcode.Tagged(errors.New("boom")))
	assert.True(t, exitcode.Tagged(xerrors.NewTagged[installpkg.InstallTag](errors.New("boom"))))
}
