// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/containerd/containerd/v2/contrib/seccomp"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/pkg/oci"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

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

var defaultUnixEnv = []string{
	"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
}

// replaceOrAppendEnvValues returns the defaults with the overrides either
// replaced by env key or appended to the list.
func replaceOrAppendEnvValues(defaults, overrides []string) []string {
	cache := make(map[string]int, len(defaults))
	results := make([]string, 0, len(defaults))

	for i, e := range defaults {
		k, _, _ := strings.Cut(e, "=")

		results = append(results, e)
		cache[k] = i
	}

	for _, value := range overrides {
		k, _, _ := strings.Cut(value, "=")

		if i, exists := cache[k]; exists {
			results[i] = value
		} else {
			results = append(results, value)
		}
	}

	return results
}

// WithImageConfigStripped is a reduced version of WithImageConfig which skips WithUser call.
//
// The function oci.WithUser has issues with deadcode elimination in containerd >= 2.3.0 due to
// a call chain that includes a call to text/template.
func WithImageConfigStripped(image oci.Image) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *specs.Spec) error {
		ic, err := image.Config(ctx)
		if err != nil {
			return err
		}

		if !images.IsConfigType(ic.MediaType) {
			return fmt.Errorf("unknown image config media type %s", ic.MediaType)
		}

		imageConfigBytes, err := content.ReadBlob(ctx, image.ContentStore(), ic)
		if err != nil {
			return err
		}

		var ociimage v1.Image

		if err = json.Unmarshal(imageConfigBytes, &ociimage); err != nil {
			return err
		}

		config := ociimage.Config

		if s.Process == nil {
			s.Process = &specs.Process{}
		}

		if s.Linux != nil {
			defaults := config.Env

			if len(defaults) == 0 {
				defaults = defaultUnixEnv
			}

			s.Process.Env = replaceOrAppendEnvValues(defaults, s.Process.Env)
			s.Process.Args = slices.Concat(config.Entrypoint, config.Cmd)

			cwd := config.WorkingDir
			if cwd == "" {
				cwd = "/"
			}

			s.Process.Cwd = cwd
		}

		return nil
	}
}
