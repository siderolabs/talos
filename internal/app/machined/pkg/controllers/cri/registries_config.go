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
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	config2 "github.com/siderolabs/talos/pkg/machinery/config/config"
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
			ID:        optional.Some(config.ActiveID),
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

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get machine config: %w", err)
		}

		imageCacheConfig, err := safe.ReaderGetByID[*cri.ImageCacheConfig](ctx, r, cri.ImageCacheConfigID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get image cache config: %w", err)
		}

		if err := safe.WriterModify(ctx, r, cri.NewRegistriesConfig(), func(res *cri.RegistriesConfig) error {
			spec := res.TypedSpec()

			spec.RegistryAuths = clearInit(spec.RegistryAuths)
			spec.RegistryMirrors = clearInit(spec.RegistryMirrors)
			spec.RegistryTLSs = clearInit(spec.RegistryTLSs)

			if cfg != nil {
				for k, v := range cfg.Config().RegistryMirrorConfigs() {
					spec.RegistryMirrors[k] = &cri.RegistryMirrorConfig{
						MirrorEndpoints: xslices.Map(
							v.Endpoints(),
							func(endpoint config2.RegistryEndpointConfig) cri.RegistryEndpointConfig {
								return cri.RegistryEndpointConfig{
									EndpointEndpoint:     endpoint.Endpoint(),
									EndpointOverridePath: endpoint.OverridePath(),
								}
							},
						),
						MirrorSkipFallback: v.SkipFallback(),
					}
				}

				for k, v := range cfg.Config().RegistryAuthConfigs() {
					spec.RegistryAuths[k] = &cri.RegistryAuthConfig{
						RegistryUsername:      v.Username(),
						RegistryPassword:      v.Password(),
						RegistryAuth:          v.Auth(),
						RegistryIdentityToken: v.IdentityToken(),
					}
				}

				for k, v := range cfg.Config().RegistryTLSConfigs() {
					spec.RegistryTLSs[k] = &cri.RegistryTLSConfig{
						TLSCA:                 v.CA(),
						TLSInsecureSkipVerify: v.InsecureSkipVerify(),
						TLSClientIdentity:     v.ClientIdentity(),
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
						[]cri.RegistryEndpointConfig{{EndpointEndpoint: "http://" + constants.RegistrydListenAddress}},
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
