// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

// RegistriesConfigController watches v1alpha1.Config, updates registry.RegistriesConfig.
type RegistriesConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *RegistriesConfigController) Name() string {
	return "cri.RegistriesConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RegistriesConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: cri.NamespaceName,
			Type:      cri.ImageCacheConfigType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *RegistriesConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: cri.RegistriesConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *RegistriesConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		r.StartTrackingOutputs()

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get machine config: %w", err)
		}

		imageCacheConfig, err := safe.ReaderGetByID[*cri.ImageCacheConfig](ctx, r, cri.ImageCacheConfigID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get image cache config: %w", err)
		}

		if err := safe.WriterModify(ctx, r, cri.NewRegistriesConfig(), func(res *cri.RegistriesConfig) error {
			spec := res.TypedSpec()

			spec.RegistryConfig = clearInit(spec.RegistryConfig)
			spec.RegistryMirrors = clearInit(spec.RegistryMirrors)

			if cfg != nil && cfg.Config().Machine() != nil {
				// This is breaking our interface abstraction, but we need to get the underlying types for protobuf
				// encoding to work correctly.
				mr := cfg.Provider().RawV1Alpha1().MachineConfig.MachineRegistries

				for k, v := range mr.RegistryConfig {
					spec.RegistryConfig[k] = makeRegistryConfig(v)
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
		}); err != nil {
			return fmt.Errorf("failed to write registries config: %w", err)
		}

		if err := safe.CleanupOutputs[*cri.RegistriesConfig](ctx, r); err != nil {
			return fmt.Errorf("failed to clean up outputs: %w", err)
		}
	}
}

func clearInit[M ~map[K]V, K comparable, V any](m M) M {
	if m == nil {
		return make(M)
	}

	clear(m)

	return m
}

func makeRegistryConfig(cfg *v1alpha1.RegistryConfig) *cri.RegistryConfig {
	result := &cri.RegistryConfig{}

	if rtls := cfg.RegistryTLS; rtls != nil {
		result.RegistryTLS = &cri.RegistryTLSConfig{
			TLSClientIdentity:     rtls.TLSClientIdentity,
			TLSCA:                 rtls.TLSCA,
			TLSInsecureSkipVerify: rtls.TLSInsecureSkipVerify,
		}
	}

	if rauth := cfg.RegistryAuth; rauth != nil {
		result.RegistryAuth = &cri.RegistryAuthConfig{
			RegistryUsername:      rauth.RegistryUsername,
			RegistryPassword:      rauth.RegistryPassword,
			RegistryAuth:          rauth.RegistryAuth,
			RegistryIdentityToken: rauth.RegistryIdentityToken,
		}
	}

	return result
}
