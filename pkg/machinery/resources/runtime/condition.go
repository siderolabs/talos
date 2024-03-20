// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/kernel"
)

// KernelParamsSetCondition implements condition which waits for the kernels to be in sync.
type KernelParamsSetCondition struct {
	state state.State
	props []*kernel.Param
}

// NewKernelParamsSetCondition builds a coondition which waits for the kernel to be in sync.
func NewKernelParamsSetCondition(state state.State, props ...*kernel.Param) *KernelParamsSetCondition {
	return &KernelParamsSetCondition{
		state: state,
		props: props,
	}
}

func (condition *KernelParamsSetCondition) String() string {
	return "kernelParams"
}

// Wait implements condition interface.
func (condition *KernelParamsSetCondition) Wait(ctx context.Context) error {
	for _, prop := range condition.props {
		if _, err := condition.state.WatchFor(
			ctx,
			resource.NewMetadata(NamespaceName, KernelParamStatusType, prop.Key, resource.VersionUndefined),
			state.WithCondition(func(r resource.Resource) (bool, error) {
				if resource.IsTombstone(r) {
					return false, nil
				}

				status := r.(*KernelParamStatus).TypedSpec()
				if status.Current != prop.Value {
					return false, nil
				}

				return true, nil
			}),
		); err != nil {
			return err
		}
	}

	return nil
}

// ExtensionServiceConfigStatusCondition implements condition which waits for extension service config to be available.
type ExtensionServiceConfigStatusCondition struct {
	state       state.State
	serviceName string
}

// NewExtensionServiceConfigStatusCondition builds a coondition which waits for extension service config to be available.
func NewExtensionServiceConfigStatusCondition(state state.State, serviceName string) *ExtensionServiceConfigStatusCondition {
	return &ExtensionServiceConfigStatusCondition{
		state:       state,
		serviceName: serviceName,
	}
}

func (condition *ExtensionServiceConfigStatusCondition) String() string {
	return "extension service config"
}

// Wait implements condition interface.
func (condition *ExtensionServiceConfigStatusCondition) Wait(ctx context.Context) error {
	_, err := condition.state.WatchFor(
		ctx,
		resource.NewMetadata(NamespaceName, ExtensionServiceConfigStatusType, condition.serviceName, resource.VersionUndefined),
		state.WithEventTypes(state.Created, state.Updated),
	)

	return err
}
