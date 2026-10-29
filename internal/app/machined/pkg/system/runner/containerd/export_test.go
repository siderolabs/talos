// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd

import (
	"github.com/containerd/containerd/v2/pkg/oci"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
)

// OCISpecOpts exposes the spec options the runner builds for an imageless container, for tests.
func OCISpecOpts(args *runner.Args, setters ...runner.Option) []oci.SpecOpts {
	return NewRunner(false, args, setters...).(*containerdRunner).newOCISpecOpts(nil) //nolint:forcetypeassert
}
