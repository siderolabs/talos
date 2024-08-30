// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// UniqueMachineTokenController provides a unique token the machine.
type UniqueMachineTokenController = transform.Controller[*runtime.MetaLoaded, *runtime.UniqueMachineToken]

// NewUniqueMachineTokenController instanciates the controller.
func NewUniqueMachineTokenController() *UniqueMachineTokenController {
	return transform.NewController(
		transform.Settings[*runtime.MetaLoaded, *runtime.UniqueMachineToken]{
			Name: "runtime.UniqueMachineTokenController",
			MapMetadataFunc: func(in *runtime.MetaLoaded) *runtime.UniqueMachineToken {
				return runtime.NewUniqueMachineToken()
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, _ *runtime.MetaLoaded, out *runtime.UniqueMachineToken) error {
				uniqueToken, err := safe.ReaderGetByID[*runtime.MetaKey](ctx, r, runtime.MetaKeyTagToID(meta.UniqueMachineToken))
				if state.IsNotFoundError(err) {
					out.TypedSpec().Token = ""

					return nil
				} else if err != nil {
					return err
				}

				out.TypedSpec().Token = uniqueToken.TypedSpec().Value

				return nil
			},
		},
		transform.WithExtraInputs(
			controller.Input{
				Namespace: runtime.NamespaceName,
				Type:      runtime.MetaKeyType,
				ID:        optional.Some(runtime.MetaKeyTagToID(meta.UniqueMachineToken)),
				Kind:      controller.InputWeak,
			},
		),
	)
}
