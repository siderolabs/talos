// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/talos-systems/talos/pkg/kernel"
	"github.com/talos-systems/talos/pkg/resources/runtime"
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
//nolint:gocyclo
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
			list, err := r.List(ctx, resource.NewMetadata(runtime.NamespaceName, runtime.KernelParamSpecType, "", resource.VersionUndefined))
			if err != nil {
				return err
			}

			touchedIDs := map[string]struct{}{}

			var errs *multierror.Error

			for _, item := range list.Items {
				spec := item.(*runtime.KernelParamSpec).TypedSpec()
				key := item.Metadata().ID()

				if err = ctrl.updateKernelParam(ctx, r, key, spec.Value); err != nil {
					if errors.Is(err, os.ErrNotExist) && spec.IgnoreErrors {
						status := runtime.NewKernelParamStatus(runtime.NamespaceName, key)

						if e := r.Modify(ctx, status, func(res resource.Resource) error {
							res.(*runtime.KernelParamStatus).TypedSpec().Unsupported = true

							return nil
						}); e != nil {
							errs = multierror.Append(errs, err)
						}
					} else {
						errs = multierror.Append(errs, err)
					}

					continue
				}

				touchedIDs[item.Metadata().ID()] = struct{}{}
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
	}
}

func (ctrl *KernelParamSpecController) updateKernelParam(ctx context.Context, r controller.Runtime, key, value string) error {
	prop := &kernel.Param{Key: key, Value: value}

	if _, ok := ctrl.defaults[key]; !ok {
		if data, err := kernel.ReadParam(prop); err == nil {
			ctrl.defaults[key] = string(data)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	if err := kernel.WriteParam(prop); err != nil {
		return err
	}

	ctrl.state[key] = value

	status := runtime.NewKernelParamStatus(runtime.NamespaceName, key)

	return r.Modify(ctx, status, func(res resource.Resource) error {
		res.(*runtime.KernelParamStatus).TypedSpec().Current = value
		res.(*runtime.KernelParamStatus).TypedSpec().Default = strings.TrimSpace(ctrl.defaults[key])

		return nil
	})
}

func (ctrl *KernelParamSpecController) resetKernelParam(ctx context.Context, r controller.Runtime, key string) error {
	var err error

	if def, ok := ctrl.defaults[key]; ok {
		err = kernel.WriteParam(&kernel.Param{
			Key:   key,
			Value: def,
		})
	} else {
		err = kernel.DeleteParam(&kernel.Param{Key: key})
	}

	if err != nil {
		return err
	}

	delete(ctrl.defaults, key)
	delete(ctrl.state, key)

	return r.Destroy(ctx, resource.NewMetadata(runtime.NamespaceName, runtime.KernelParamStatusType, key, resource.VersionUndefined))
}
