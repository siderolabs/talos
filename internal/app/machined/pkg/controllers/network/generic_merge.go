// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources"
)

type genericMergeFunc[T typed.DeepCopyable[T], E typed.Extension] func(logger *zap.Logger, in safe.List[*typed.Resource[T, E]]) map[resource.ID]*T

// DeepCopyRedactable combines the DeepCopyable and Redactable interfaces for generic type T.
type DeepCopyRedactable[T any] interface {
	typed.DeepCopyable[T]
	resources.RedactableSpec[T]
}

// GenericMergeController initializes a generic merge controller for network resources.
func GenericMergeController[T DeepCopyRedactable[T], E typed.Extension](namespaceIn, namespaceOut resource.Namespace, mergeFunc genericMergeFunc[T, E]) controller.Controller {
	var zeroE E

	controllerName := strings.ReplaceAll(zeroE.ResourceDefinition().Type, "Spec", "MergeController")

	return &genericMergeController[T, E]{
		controllerName: controllerName,
		resourceType:   zeroE.ResourceDefinition().Type,
		namespaceIn:    namespaceIn,
		namespaceOut:   namespaceOut,
		mergeFunc:      mergeFunc,
	}
}

type genericMergeController[T DeepCopyRedactable[T], E typed.Extension] struct {
	controllerName string
	resourceType   resource.Type
	namespaceIn    resource.Namespace
	namespaceOut   resource.Namespace
	mergeFunc      genericMergeFunc[T, E]
}

func (ctrl *genericMergeController[T, E]) Name() string {
	return ctrl.controllerName
}

func (ctrl *genericMergeController[T, E]) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: ctrl.namespaceIn,
			Type:      ctrl.resourceType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: ctrl.namespaceOut,
			Type:      ctrl.resourceType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

func (ctrl *genericMergeController[T, E]) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: ctrl.resourceType,
			Kind: controller.OutputShared,
		},
	}
}

//nolint:gocyclo
func (ctrl *genericMergeController[T, E]) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		type R = typed.Resource[T, E]

		// list source network configuration resources
		in, err := safe.ReaderList[*R](ctx, r, resource.NewMetadata(ctrl.namespaceIn, ctrl.resourceType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing source network resources: %w", err)
		}

		merged := ctrl.mergeFunc(logger, in)

		// cleanup resources, detecting conflicts on the way
		out, err := safe.ReaderList[*R](ctx, r, resource.NewMetadata(ctrl.namespaceOut, ctrl.resourceType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing output resources: %w", err)
		}

		for res := range out.All() {
			shouldBeDestroyed := false
			if _, ok := merged[res.Metadata().ID()]; !ok {
				shouldBeDestroyed = true
			}

			isTearingDown := res.Metadata().Phase() == resource.PhaseTearingDown

			if shouldBeDestroyed || isTearingDown {
				var okToDestroy bool

				okToDestroy, err = r.Teardown(ctx, res.Metadata())
				if err != nil {
					return fmt.Errorf("error cleaning up addresses: %w", err)
				}

				if okToDestroy {
					if err = r.Destroy(ctx, res.Metadata()); err != nil {
						return fmt.Errorf("error cleaning up addresses: %w", err)
					}
				} else if !shouldBeDestroyed {
					// resource is not ready to be destroyed yet, skip it
					delete(merged, res.Metadata().ID())
				}
			}
		}

		var zeroT T

		for id, spec := range merged {
			md := resource.NewMetadata(ctrl.namespaceOut, ctrl.resourceType, id, resource.VersionUndefined)

			if err = safe.WriterModify(ctx, r,
				typed.NewResource[T, E](md, zeroT),
				func(r *R) error {
					*r.TypedSpec() = *spec

					return nil
				}); err != nil {
				return fmt.Errorf("error updating resource: %w", err)
			}

			specRedacted := (*spec).RedactSecrets(md)

			logger.Debug("merged spec", zap.String("id", id), zap.Any("spec", specRedacted))
		}

		r.ResetRestartBackoff()
	}
}
