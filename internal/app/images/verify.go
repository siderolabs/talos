// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package images

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/pkg/containers/image"
	"github.com/siderolabs/talos/internal/pkg/containers/image/verify"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

// Verify an image signature against the configured verification policy.
//
// This endpoint is called by containerd before unpacking an image to ensure
// the image meets the verification requirements configured in the machine config.
//
// If no verification policy is configured, all images are allowed by default.
func (svc *Service) Verify(ctx context.Context, req *machine.ImageServiceVerifyRequest) (*machine.ImageServiceVerifyResponse, error) {
	// build resolver with custom auth if credentials are provided in the request
	var opts []func(*cri.RegistriesConfigSpec)

	if req.Credentials != nil {
		opts = append(opts, func(spec *cri.RegistriesConfigSpec) {
			if spec.RegistryAuths == nil {
				spec.RegistryAuths = make(map[string]*cri.RegistryAuthConfig)
			}

			spec.RegistryAuths[req.GetCredentials().GetHost()] = &cri.RegistryAuthConfig{
				RegistryUsername: req.GetCredentials().Username,
				RegistryPassword: req.GetCredentials().Password,
			}
		})
	}

	registries, err := cri.RegistryBuilder(svc.controller.Runtime().State().V1Alpha2().Resources(), opts...)(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to build registry configuration: %s", err)
	}

	resolver := image.NewResolver(registries)
	tagFetcher := image.NewTagFetcher(registries)

	return verify.ImageSignature(ctx, svc.logger, svc.controller.Runtime().State().V1Alpha2().Resources(), resolver, tagFetcher, req.GetImageRef())
}
