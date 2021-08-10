// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/talos-systems/talos/pkg/kernel"
	"github.com/talos-systems/talos/pkg/kernel/kspp"
	"github.com/talos-systems/talos/pkg/resources/runtime"
)

func TestCondition(t *testing.T) {
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(ctxCancel)

	t.Parallel()

	for _, tt := range []struct {
		Name               string
		ActualKernelParams []*kernel.Param
		AwaitKernelParams  []*kernel.Param
		Succeeds           bool
	}{
		{
			Name: "okay",
			ActualKernelParams: []*kernel.Param{
				{
					Key:   "kernel.kptr_restrict",
					Value: "1",
				},
			},
			AwaitKernelParams: []*kernel.Param{
				{
					Key:   "kernel.kptr_restrict",
					Value: "1",
				},
			},
			Succeeds: true,
		},
		{
			Name:               "timeout",
			ActualKernelParams: []*kernel.Param{},
			AwaitKernelParams: []*kernel.Param{
				{
					Key:   "kernel.kptr_restrict",
					Value: "1",
				},
			},
			Succeeds: false,
		},
		{
			Name: "value differs",
			ActualKernelParams: []*kernel.Param{
				{
					Key:   "kernel.kptr_restrict",
					Value: "0",
				},
			},
			AwaitKernelParams: []*kernel.Param{
				{
					Key:   "kernel.kptr_restrict",
					Value: "1",
				},
			},
			Succeeds: false,
		},
		{
			Name:               "multiple values",
			ActualKernelParams: kspp.GetKernelParams(),
			AwaitKernelParams:  kspp.GetKernelParams(),
			Succeeds:           true,
		},
	} {
		tt := tt

		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			state := state.WrapCore(namespaced.NewState(inmem.Build))

			for _, prop := range tt.ActualKernelParams {
				status := runtime.NewKernelParamStatus(runtime.NamespaceName, prop.Key)
				*status.TypedSpec() = runtime.KernelParamStatusSpec{
					Current: prop.Value,
				}

				require.NoError(t, state.Create(ctx, status))
			}

			err := runtime.NewKernelParamsSetCondition(state, tt.AwaitKernelParams...).Wait(ctx)

			if tt.Succeeds {
				assert.NoError(t, err)
			} else {
				assert.True(t, errors.Is(err, context.DeadlineExceeded), "error is %v", err)
			}
		})
	}
}
