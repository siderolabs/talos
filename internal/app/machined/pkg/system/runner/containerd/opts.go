// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd

import (
	"context"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/contrib/seccomp"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// WithMemoryLimit sets the linux resource memory limit field.
func WithMemoryLimit(limit int64) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		s.Linux.Resources.Memory = &specs.LinuxMemory{
			Limit: &limit,
			// DisableOOMKiller: &disable,
		}

		return nil
	}
}

// WithRootfsPropagation sets the root filesystem propagation.
func WithRootfsPropagation(rp string) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		s.Linux.RootfsPropagation = rp

		return nil
	}
}

// WithOOMScoreAdj sets the oom score.
func WithOOMScoreAdj(score int) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		s.Process.OOMScoreAdj = &score

		return nil
	}
}

// WithCustomSeccompProfile allows to override default seccomp profile.
func WithCustomSeccompProfile(override func(*specs.LinuxSeccomp)) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		s.Linux.Seccomp = seccomp.DefaultProfile(s)

		override(s.Linux.Seccomp)

		return nil
	}
}
