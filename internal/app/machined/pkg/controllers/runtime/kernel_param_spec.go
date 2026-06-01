// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	krnl "github.com/siderolabs/talos/pkg/kernel"
	"github.com/siderolabs/talos/pkg/kernel/kspp"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// KernelParamSpecController watches KernelParamSpecs, sets/resets kernel params.
type KernelParamSpecController struct {
	defaults map[string]string
	state    map[string]string
}

// Name implements controller.Controller interface.
func (ctrl *KernelParamSpecController) Name() string {
	return "runtime.KernelParamSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KernelParamSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.KernelParamDefaultSpecType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.KernelParamSpecType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KernelParamSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.KernelParamStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *KernelParamSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.state == nil {
		ctrl.state = map[string]string{}
	}

	if ctrl.defaults == nil {
		ctrl.defaults = map[string]string{}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			ksppParams := map[string]struct{}{}

			for _, param := range kspp.GetKernelParams() {
				ksppParams[param.Key] = struct{}{}
			}

			defaults, err := r.List(ctx, resource.NewMetadata(runtime.NamespaceName, runtime.KernelParamDefaultSpecType, "", resource.VersionUndefined))
			if err != nil {
				return err
			}

			configs, err := r.List(ctx, resource.NewMetadata(runtime.NamespaceName, runtime.KernelParamSpecType, "", resource.VersionUndefined))
			if err != nil {
				return err
			}

			configsCounts := len(configs.Items)

			list := slices.Concat(configs.Items, defaults.Items)

			touchedIDs := map[string]string{}

			var errs *multierror.Error

			for i, item := range list {
				spec := item.(runtime.KernelParam).TypedSpec()
				id := item.Metadata().ID()

				if value, duplicate := touchedIDs[id]; i >= configsCounts && duplicate {
					if _, ok := ksppParams[id]; ok {
						logger.Warn("overriding KSPP enforced parameter, this is not recommended", zap.String("key", id), zap.String("value", value))
					}

					continue
				}

				if err = ctrl.updateKernelParam(ctx, r, id, spec.Value); err != nil {
					if errors.Is(err, os.ErrNotExist) && spec.IgnoreErrors {
						status := runtime.NewKernelParamStatus(runtime.NamespaceName, id)

						if e := safe.WriterModify(ctx, r, status, func(res *runtime.KernelParamStatus) error {
							res.TypedSpec().Unsupported = true

							return nil
						}); e != nil {
							errs = multierror.Append(errs, e)
						}
					} else {
						errs = multierror.Append(errs, err)
					}

					continue
				}

				touchedIDs[id] = spec.Value
			}

			for key := range ctrl.state {
				if _, ok := touchedIDs[key]; ok {
					continue
				}

				if err = ctrl.resetKernelParam(ctx, r, key); err != nil {
					errs = multierror.Append(errs, err)
				}
			}

			if errs != nil {
				return errs
			}
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *KernelParamSpecController) updateKernelParam(ctx context.Context, r controller.Runtime, key, value string) error {
	prop := &kernel.Param{Key: key, Value: value}

	if _, ok := ctrl.defaults[key]; !ok {
		if data, err := krnl.ReadParam(prop); err == nil {
			ctrl.defaults[key] = string(data)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	if err := krnl.WriteParam(prop); err != nil {
		return err
	}

	ctrl.state[key] = value

	status := runtime.NewKernelParamStatus(runtime.NamespaceName, key)

	return safe.WriterModify(ctx, r, status, func(res *runtime.KernelParamStatus) error {
		res.TypedSpec().Current = value
		res.TypedSpec().Default = strings.TrimSpace(ctrl.defaults[key])

		return nil
	})
}

func (ctrl *KernelParamSpecController) resetKernelParam(ctx context.Context, r controller.Runtime, key string) error {
	var err error

	if def, ok := ctrl.defaults[key]; ok {
		err = krnl.WriteParam(&kernel.Param{Key: key, Value: def})
	} else {
		err = krnl.DeleteParam(&kernel.Param{Key: key})
	}

	if err != nil {
		return err
	}

	delete(ctrl.defaults, key)
	delete(ctrl.state, key)

	return r.Destroy(ctx, resource.NewMetadata(runtime.NamespaceName, runtime.KernelParamStatusType, key, resource.VersionUndefined))
}
