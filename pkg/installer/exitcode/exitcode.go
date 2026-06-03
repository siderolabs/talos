// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package exitcode resolves installer/imager errors to stable process exit codes.
package exitcode

import (
	"github.com/siderolabs/gen/xerrors"

	installpkg "github.com/siderolabs/talos/cmd/installer/pkg/install"
	pkgimager "github.com/siderolabs/talos/pkg/imager"
	profilepkg "github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Tagged reports whether err already carries a known installer/imager exit tag.
func Tagged(err error) bool {
	return err != nil && Resolve(err) != constants.ExitUnknownError
}

// Resolve returns stable installer/imager exit code for err.
//
// Priority order is explicit and deterministic. More specific caller-actionable
// categories win over broader execution failures.
func Resolve(err error) int {
	if err == nil {
		return constants.ExitSuccess
	}

	switch {
	case xerrors.TagIs[profilepkg.InvalidInputTag](err),
		xerrors.TagIs[pkgimager.InvalidInputTag](err),
		xerrors.TagIs[installpkg.InvalidInputTag](err):
		return constants.ExitInvalidInput
	case xerrors.TagIs[profilepkg.UnsupportedTag](err),
		xerrors.TagIs[pkgimager.UnsupportedTag](err):
		return constants.ExitUnsupported
	case xerrors.TagIs[installpkg.EnvironmentTag](err):
		return constants.ExitEnvironment
	case xerrors.TagIs[installpkg.DependencyTag](err),
		xerrors.TagIs[pkgimager.DependencyTag](err):
		return constants.ExitDependency
	case xerrors.TagIs[pkgimager.IOTag](err):
		return constants.ExitIO
	case xerrors.TagIs[installpkg.InstallTag](err),
		xerrors.TagIs[pkgimager.InstallTag](err):
		return constants.ExitInstall
	default:
		return constants.ExitUnknownError
	}
}
