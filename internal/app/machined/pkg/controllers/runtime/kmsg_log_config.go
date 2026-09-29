// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"maps"
	"net/url"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// KmsgLogConfigController generates configuration for kmsg log delivery.
type KmsgLogConfigController struct {
	Cmdline *procfs.Cmdline
}

// Name implements controller.Controller interface.
func (ctrl *KmsgLogConfigController) Name() string {
	return "runtime.KmsgLogConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KmsgLogConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KmsgLogConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.KmsgLogConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *KmsgLogConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) (err error) {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		var taggedDestinations []runtime.KmsgLogDestination

		destinationIdx := map[string]int{}

		// addDestination deduplicates the destinations by the endpoint: if the same endpoint is specified
		// more than once (e.g. both in the kernel args and the machine config), a single destination is kept
		// with the extra tags merged together.
		addDestination := func(endpoint *url.URL, extraTags map[string]string) {
			idx, ok := destinationIdx[endpoint.String()]
			if !ok {
				destinationIdx[endpoint.String()] = len(taggedDestinations)

				taggedDestinations = append(taggedDestinations, runtime.KmsgLogDestination{
					Endpoint:  endpoint,
					ExtraTags: maps.Clone(extraTags),
				})

				return
			}

			if len(extraTags) == 0 {
				return
			}

			if taggedDestinations[idx].ExtraTags == nil {
				taggedDestinations[idx].ExtraTags = map[string]string{}
			}

			maps.Copy(taggedDestinations[idx].ExtraTags, extraTags)
		}

		if ctrl.Cmdline != nil {
			if val := ctrl.Cmdline.Get(constants.KernelParamLoggingKernel).First(); val != nil {
				destURL, err := url.Parse(*val)
				if err != nil {
					return fmt.Errorf("error parsing %q: %w", constants.KernelParamLoggingKernel, err)
				}

				addDestination(destURL, nil)
			}
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting machine config: %w", err)
		}

		if cfg != nil {
			for _, destination := range cfg.Config().Runtime().KmsgLogDestinations() {
				addDestination(destination.Endpoint, destination.ExtraTags)
			}
		}

		r.StartTrackingOutputs()

		if len(taggedDestinations) > 0 {
			if err = safe.WriterModify(ctx, r, runtime.NewKmsgLogConfig(), func(cfg *runtime.KmsgLogConfig) error {
				// Keep the legacy destination list populated for older clients. Newer clients
				// prefer TaggedDestinations to retain the tags associated with each endpoint.
				cfg.TypedSpec().Destinations = xslices.Map(taggedDestinations, func(d runtime.KmsgLogDestination) *url.URL { return d.Endpoint })
				cfg.TypedSpec().TaggedDestinations = taggedDestinations

				return nil
			}); err != nil {
				return fmt.Errorf("error updating kmsg log config: %w", err)
			}
		}

		if err = safe.CleanupOutputs[*runtime.KmsgLogConfig](ctx, r); err != nil {
			return err
		}
	}
}
