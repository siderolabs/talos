// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:scopelint,dupl,testpackage
package v1alpha1

import (
	"context"
	"reflect"
	"strconv"
	"testing"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

func TestNewController(t *testing.T) {
	type args struct {
		b []byte
	}

	tests := []struct {
		name    string
		args    args
		want    *Controller
		wantErr bool
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewController(tt.args.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewController() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewController() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_Run(t *testing.T) {
	type fields struct {
		r         *Runtime
		s         *Sequencer
		semaphore int32
	}

	type args struct {
		seq  runtime.Sequence
		data interface{}
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				r:         tt.fields.r,
				s:         tt.fields.s,
				semaphore: tt.fields.semaphore,
			}
			if err := c.Run(ctx, tt.args.seq, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("Controller.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestController_Runtime(t *testing.T) {
	type fields struct {
		r         *Runtime
		s         *Sequencer
		semaphore int32
	}

	tests := []struct {
		name   string
		fields fields
		want   runtime.Runtime
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				r:         tt.fields.r,
				s:         tt.fields.s,
				semaphore: tt.fields.semaphore,
			}
			if got := c.Runtime(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Controller.Runtime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_Sequencer(t *testing.T) {
	type fields struct {
		r         *Runtime
		s         *Sequencer
		semaphore int32
	}

	tests := []struct {
		name   string
		fields fields
		want   runtime.Sequencer
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				r:         tt.fields.r,
				s:         tt.fields.s,
				semaphore: tt.fields.semaphore,
			}
			if got := c.Sequencer(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Controller.Sequencer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_ListenForEvents(t *testing.T) {
	type fields struct {
		r         *Runtime
		s         *Sequencer
		semaphore int32
	}

	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				r:         tt.fields.r,
				s:         tt.fields.s,
				semaphore: tt.fields.semaphore,
			}
			if err := c.ListenForEvents(ctx); (err != nil) != tt.wantErr {
				t.Errorf("Controller.ListenForEvents() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestController_TryLock(t *testing.T) {
	type fields struct {
		r         *Runtime
		s         *Sequencer
		semaphore int32
	}

	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				r:         tt.fields.r,
				s:         tt.fields.s,
				semaphore: tt.fields.semaphore,
			}
			if got := c.TryLock(); got != tt.want {
				t.Errorf("Controller.TryLock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_Unlock(t *testing.T) {
	type fields struct {
		r         *Runtime
		s         *Sequencer
		semaphore int32
	}

	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				r:         tt.fields.r,
				s:         tt.fields.s,
				semaphore: tt.fields.semaphore,
			}
			if got := c.Unlock(); got != tt.want {
				t.Errorf("Controller.Unlock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_run(t *testing.T) {
	type fields struct {
		r         *Runtime
		s         *Sequencer
		semaphore int32
	}

	type args struct {
		seq    runtime.Sequence
		phases []runtime.Phase
		data   interface{}
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				r:         tt.fields.r,
				s:         tt.fields.s,
				semaphore: tt.fields.semaphore,
			}
			if err := c.run(ctx, tt.args.seq, tt.args.phases, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("Controller.run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestController_runPhase(t *testing.T) {
	type fields struct {
		r         *Runtime
		s         *Sequencer
		semaphore int32
	}

	type args struct {
		phase runtime.Phase
		seq   runtime.Sequence
		data  interface{}
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				r:         tt.fields.r,
				s:         tt.fields.s,
				semaphore: tt.fields.semaphore,
			}
			if err := c.runPhase(ctx, tt.args.phase, tt.args.seq, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("Controller.runPhase() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestController_runTask(t *testing.T) {
	type fields struct {
		r         *Runtime
		s         *Sequencer
		semaphore int32
	}

	type args struct {
		n    int
		f    runtime.TaskSetupFunc
		seq  runtime.Sequence
		data interface{}
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				r:         tt.fields.r,
				s:         tt.fields.s,
				semaphore: tt.fields.semaphore,
			}
			if err := c.runTask(ctx, strconv.Itoa(tt.args.n), tt.args.f, tt.args.seq, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("Controller.runTask() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestController_phases(t *testing.T) {
	type fields struct {
		r         *Runtime
		s         *Sequencer
		semaphore int32
	}

	type args struct {
		seq  runtime.Sequence
		data interface{}
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []runtime.Phase
		wantErr bool
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				r:         tt.fields.r,
				s:         tt.fields.s,
				semaphore: tt.fields.semaphore,
			}
			got, err := c.phases(tt.args.seq, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Controller.phases() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Controller.phases() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_waitForUSBDelay(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := waitForUSBDelay(); (err != nil) != tt.wantErr {
				t.Errorf("waitForUSBDelay() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
