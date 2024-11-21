// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

// RegistriesConfigController watches v1alpha1.Config, updates registry.RegistriesConfig.
type RegistriesConfigController = transform.Controller[*config.MachineConfig, *cri.RegistriesConfig]

// NewRegistriesConfigController creates new config controller.
//
//nolint:gocyclo
func NewRegistriesConfigController() *RegistriesConfigController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *cri.RegistriesConfig]{
			Name: "cri.RegistriesConfigController",
			MapMetadataOptionalFunc: func(cfg *config.MachineConfig) optional.Optional[*cri.RegistriesConfig] {
				if cfg.Metadata().ID() != config.V1Alpha1ID {
					return optional.None[*cri.RegistriesConfig]()
				}

				return optional.Some(cri.NewRegistriesConfig())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, cfg *config.MachineConfig, res *cri.RegistriesConfig) error {
				imageCacheConfig, err := safe.ReaderGetByID[*cri.ImageCacheConfig](ctx, r, cri.ImageCacheConfigID)
				if err != nil && !state.IsNotFoundError(err) {
					return fmt.Errorf("failed to get image cache config: %w", err)
				}

				spec := res.TypedSpec()

				spec.RegistryConfig = clearInit(spec.RegistryConfig)
				spec.RegistryMirrors = clearInit(spec.RegistryMirrors)

				if cfg != nil && cfg.Config().Machine() != nil {
					// This is breaking our interface abstraction, but we need to get the underlying types for protobuf
					// encoding to work correctly.
					mr := cfg.Provider().RawV1Alpha1().MachineConfig.MachineRegistries

					for k, v := range mr.RegistryConfig {
						spec.RegistryConfig[k] = &cri.RegistryConfig{
							RegistryTLS: &cri.RegistryTLSConfig{
								TLSClientIdentity:     v.RegistryTLS.TLSClientIdentity,
								TLSCA:                 v.RegistryTLS.TLSCA,
								TLSInsecureSkipVerify: v.RegistryTLS.TLSInsecureSkipVerify,
							},
							RegistryAuth: &cri.RegistryAuthConfig{
								RegistryUsername:      v.RegistryAuth.RegistryUsername,
								RegistryPassword:      v.RegistryAuth.RegistryPassword,
								RegistryAuth:          v.RegistryAuth.RegistryAuth,
								RegistryIdentityToken: v.RegistryAuth.RegistryIdentityToken,
							},
						}
					}

					for k, v := range mr.RegistryMirrors {
						spec.RegistryMirrors[k] = &cri.RegistryMirrorConfig{
							MirrorEndpoints:    v.MirrorEndpoints,
							MirrorOverridePath: v.MirrorOverridePath,
							MirrorSkipFallback: v.MirrorSkipFallback,
						}
					}
				}

				if imageCacheConfig != nil && imageCacheConfig.TypedSpec().Status == cri.ImageCacheStatusReady {
					// if the '*' was configured, we just use it, otherwise create it so that we can inject the registryd
					if _, hasStar := spec.RegistryMirrors["*"]; !hasStar {
						spec.RegistryMirrors["*"] = &cri.RegistryMirrorConfig{}
					}

					// inject the registryd mirror endpoint as the first one for all registries
					for registry := range spec.RegistryMirrors {
						spec.RegistryMirrors[registry].MirrorEndpoints = append(
							[]string{"http://" + constants.RegistrydListenAddress},
							spec.RegistryMirrors[registry].MirrorEndpoints...,
						)
					}
				}

				return nil
			},
		},
		transform.WithExtraInputs(
			controller.Input{
				Namespace: cri.NamespaceName,
				Type:      cri.ImageCacheConfigType,
				ID:        optional.Some(cri.ImageCacheConfigID),
				Kind:      controller.InputWeak,
			},
		),
	)
}

func clearInit[M ~map[K]V, K comparable, V any](m M) M {
	if m == nil {
		return make(M)
	}

	clear(m)

	return m
}
