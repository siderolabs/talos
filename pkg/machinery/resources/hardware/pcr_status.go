// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// PCRStatusType is type of PCRStatus resource.
const PCRStatusType = resource.Type("PCRStatuses.hardware.talos.dev")

// PCRStatus resource holds node PCRStatus information.
type PCRStatus = typed.Resource[PCRStatusSpec, PCRStatusExtension]

// PCRStatusSpec represents a single PCR status.
//
// The resource is created when the PCR is ready to be used, and
// torn down/destroyed as the PCR value is extended to prevent it from being
// used.
//
//gotagsrewrite:gen
type PCRStatusSpec struct{}

// NewPCCRStatus initializes a PCRStatus resource.
func NewPCCRStatus(pcr int) *PCRStatus {
	return typed.NewResource[PCRStatusSpec, PCRStatusExtension](
		resource.NewMetadata(NamespaceName, PCRStatusType, strconv.Itoa(pcr), resource.VersionUndefined),
		PCRStatusSpec{},
	)
}

// PCRStatusExtension provides auxiliary methods for PCRStatus info.
type PCRStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (PCRStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             PCRStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

// FinalizerState is a minimal interface for state that supports finalizers.
type FinalizerState interface {
	AddFinalizer(context.Context, resource.Pointer, ...resource.Finalizer) error
	RemoveFinalizer(context.Context, resource.Pointer, ...resource.Finalizer) error
}

// LockPCRStatus locks the PCR status resource.
func LockPCRStatus(st FinalizerState, pcr int, finalizerName string) func(context.Context, func() error) error {
	return func(ctx context.Context, fn func() error) error {
		pcrStatus := NewPCCRStatus(pcr)

		if err := st.AddFinalizer(ctx, pcrStatus.Metadata(), finalizerName); err != nil {
			if state.IsNotFoundError(err) {
				return fmt.Errorf("failed to lock PCR %d, as it is in the wrong state, a reboot might be required", pcr)
			}

			return fmt.Errorf("failed to lock PCR %d: %w", pcr, err)
		}

		fnErr := fn()

		if err := st.RemoveFinalizer(ctx, pcrStatus.Metadata(), finalizerName); err != nil {
			fnErr = errors.Join(fnErr, fmt.Errorf("failed to unlock PCR %d: %w", pcr, err))
		}

		return fnErr
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(PCRStatusType, &PCRStatus{})
	if err != nil {
		panic(err)
	}
}
