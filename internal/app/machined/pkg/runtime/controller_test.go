// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: scopelint,dupl
package runtime

import (
	"fmt"
	"net"
	"testing"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/api/machine"
)

type MockSuccessfulSequencer struct{}

// Boot is a mock method that overrides the embedded sequencer's Boot method.
func (s *MockSuccessfulSequencer) Boot() []Phase {
	return []Phase{
		&MockSuccessfulPhase{},
	}
}

// Initialize is a mock method that overrides the embedded sequencer's Initialize method.
func (s *MockSuccessfulSequencer) Initialize() []Phase {
	return []Phase{
		&MockSuccessfulPhase{},
	}
}

// Shutdown is a mock method that overrides the embedded sequencer's Shutdown method.
func (s *MockSuccessfulSequencer) Shutdown() []Phase {
	return []Phase{
		&MockSuccessfulPhase{},
	}
}

// Upgrade is a mock method that overrides the embedded sequencer's Upgrade method.
func (s *MockSuccessfulSequencer) Upgrade(req *machine.UpgradeRequest) []Phase {
	return []Phase{
		&MockSuccessfulPhase{},
	}
}

// Reboot is a mock method that overrides the embedded sequencer's Reboot method.
func (s *MockSuccessfulSequencer) Reboot() []Phase {
	return []Phase{
		&MockSuccessfulPhase{},
	}
}

// Reset is a mock method that overrides the embedded sequencer's Reset method.
func (s *MockSuccessfulSequencer) Reset(req *machine.ResetRequest) []Phase {
	return []Phase{
		&MockSuccessfulPhase{},
	}
}

type MockUnsuccessfulSequencer struct{}

// Boot is a mock method that overrides the embedded sequencer's Boot method.
func (s *MockUnsuccessfulSequencer) Boot() []Phase {
	return []Phase{
		&MockUnsuccessfulPhase{},
	}
}

// Initialize is a mock method that overrides the embedded sequencer's Initialize method.
func (s *MockUnsuccessfulSequencer) Initialize() []Phase {
	return []Phase{
		&MockUnsuccessfulPhase{},
	}
}

// Shutdown is a mock method that overrides the embedded sequencer's Shutdown method.
func (s *MockUnsuccessfulSequencer) Shutdown() []Phase {
	return []Phase{
		&MockUnsuccessfulPhase{},
	}
}

// Upgrade is a mock method that overrides the embedded sequencer's Upgrade method.
func (s *MockUnsuccessfulSequencer) Upgrade(req *machine.UpgradeRequest) []Phase {
	return []Phase{
		&MockUnsuccessfulPhase{},
	}
}

// Reboot is a mock method that overrides the embedded sequencer's Reboot method.
func (s *MockUnsuccessfulSequencer) Reboot() []Phase {
	return []Phase{
		&MockUnsuccessfulPhase{},
	}
}

// Reset is a mock method that overrides the embedded sequencer's Reset method.
func (s *MockUnsuccessfulSequencer) Reset(req *machine.ResetRequest) []Phase {
	return []Phase{
		&MockUnsuccessfulPhase{},
	}
}

type MockSuccessfulPhase struct{}

func (*MockSuccessfulPhase) Tasks() []Task {
	return []Task{&MockSuccessfulTask{}}
}

type MockUnsuccessfulPhase struct{}

func (*MockUnsuccessfulPhase) Tasks() []Task {
	return []Task{&MockUnsuccessfulTask{}}
}

type MockSuccessfulTask struct{}

func (*MockSuccessfulTask) Func(Mode) TaskFunc {
	return func(Runtime) error {
		return nil
	}
}

type MockUnsuccessfulTask struct{}

func (*MockUnsuccessfulTask) Func(Mode) TaskFunc {
	return func(Runtime) error { return fmt.Errorf("error") }
}

type MockPlatform struct{}

func (*MockPlatform) Name() string {
	return "mock"
}

func (*MockPlatform) Configuration() ([]byte, error) {
	return nil, nil
}

func (*MockPlatform) ExternalIPs() ([]net.IP, error) {
	return nil, nil
}

func (*MockPlatform) Hostname() ([]byte, error) {
	return nil, nil
}

func (*MockPlatform) Mode() Mode {
	return Metal
}

func (*MockPlatform) KernelArgs() procfs.Parameters {
	return procfs.Parameters{}
}

type MockConfigurator struct{}

func (*MockConfigurator) Version() string {
	return ""
}

func (*MockConfigurator) Debug() bool {
	return false
}

func (*MockConfigurator) Persist() bool {
	return false
}

func (*MockConfigurator) Machine() Machine {
	return nil
}

func (*MockConfigurator) Cluster() Cluster {
	return nil
}

func (*MockConfigurator) Validate(Mode) error {
	return nil
}

func (*MockConfigurator) String() (string, error) {
	return "", nil
}

func (*MockConfigurator) Bytes() ([]byte, error) {
	return nil, nil
}

type MockRuntime struct{}

func (*MockRuntime) Platform() Platform {
	return &MockPlatform{}
}

func (*MockRuntime) Config() Configurator {
	return &MockConfigurator{}
}

func (*MockRuntime) Sequence() Sequence {
	return Noop
}

func TestController_Run(t *testing.T) {
	type fields struct {
		Sequencer Sequencer
		Runtime   Runtime
		semaphore int32
	}

	type args struct {
		seq  Sequence
		data interface{}
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "boot",
			fields: fields{
				Sequencer: &MockSuccessfulSequencer{},
				Runtime:   &MockRuntime{},
				semaphore: 0,
			},
			args: args{
				seq:  Boot,
				data: nil,
			},
			wantErr: false,
		},
		{
			name: "initialize",
			fields: fields{
				Sequencer: &MockSuccessfulSequencer{},
				Runtime:   &MockRuntime{},
				semaphore: 0,
			},
			args: args{
				seq:  Initialize,
				data: nil,
			},
			wantErr: false,
		},
		{
			name: "shutdown",
			fields: fields{
				Sequencer: &MockSuccessfulSequencer{},
				Runtime:   &MockRuntime{},
				semaphore: 0,
			},
			args: args{
				seq:  Shutdown,
				data: nil,
			},
			wantErr: false,
		},
		{
			name: "upgrade with valid data",
			fields: fields{
				Sequencer: &MockSuccessfulSequencer{},
				Runtime:   &MockRuntime{},
				semaphore: 0,
			},
			args: args{
				seq:  Upgrade,
				data: &machine.UpgradeRequest{},
			},
			wantErr: false,
		},
		{
			name: "upgrade with invalid data",
			fields: fields{
				Sequencer: &MockSuccessfulSequencer{},
				Runtime:   &MockRuntime{},
				semaphore: 0,
			},
			args: args{
				seq:  Upgrade,
				data: nil,
			},
			wantErr: true,
		},
		{
			name: "upgrade with lock",
			fields: fields{
				Sequencer: &MockSuccessfulSequencer{},
				Runtime:   &MockRuntime{},
				semaphore: 1,
			},
			args: args{
				seq:  Upgrade,
				data: &machine.UpgradeRequest{},
			},
			wantErr: true,
		},
		{
			name: "reset with valid data",
			fields: fields{
				Sequencer: &MockSuccessfulSequencer{},
				Runtime:   &MockRuntime{},
				semaphore: 0,
			},
			args: args{
				seq:  Reset,
				data: &machine.ResetRequest{},
			},
			wantErr: false,
		},
		{
			name: "reset with invalid data",
			fields: fields{
				Sequencer: &MockSuccessfulSequencer{},
				Runtime:   &MockRuntime{},
				semaphore: 0,
			},
			args: args{
				seq:  Reset,
				data: nil,
			},
			wantErr: true,
		},
		{
			name: "unsuccessful phase",
			fields: fields{
				Sequencer: &MockUnsuccessfulSequencer{},
				Runtime:   &MockRuntime{},
				semaphore: 0,
			},
			args: args{
				seq:  Boot,
				data: nil,
			},
			wantErr: true,
		},
		{
			name: "undefined runtime",
			fields: fields{
				Sequencer: &MockSuccessfulSequencer{},
				Runtime:   nil,
				semaphore: 0,
			},
			args: args{
				seq:  Boot,
				data: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				Sequencer: tt.fields.Sequencer,
				Runtime:   tt.fields.Runtime,
				semaphore: tt.fields.semaphore,
			}
			t.Logf("c.Sequencer: %v", c.Sequencer)
			if err := c.Run(tt.args.seq, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("Controller.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestController_runPhase(t *testing.T) {
	type fields struct {
		Sequencer Sequencer
		Runtime   Runtime
		semaphore int32
	}

	type args struct {
		phase Phase
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "successful phase",
			fields: fields{
				Sequencer: &MockSuccessfulSequencer{},
				Runtime:   &MockRuntime{},
				semaphore: 0,
			},
			args: args{
				phase: &MockSuccessfulPhase{},
			},
			wantErr: false,
		},
		{
			name: "unsuccessful phase",
			fields: fields{
				Sequencer: &MockSuccessfulSequencer{},
				Runtime:   &MockRuntime{},
				semaphore: 0,
			},
			args: args{
				phase: &MockUnsuccessfulPhase{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				Sequencer: tt.fields.Sequencer,
				Runtime:   tt.fields.Runtime,
				semaphore: tt.fields.semaphore,
			}
			if err := c.runPhase(tt.args.phase); (err != nil) != tt.wantErr {
				t.Errorf("Controller.runPhase() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestController_runTask(t *testing.T) {
	type fields struct {
		Sequencer Sequencer
		Runtime   Runtime
		semaphore int32
	}

	type args struct {
		t Task
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "successful task",
			fields: fields{
				Sequencer: &MockSuccessfulSequencer{},
				Runtime:   &MockRuntime{},
				semaphore: 0,
			},
			args: args{
				t: &MockSuccessfulTask{},
			},
			wantErr: false,
		},
		{
			name: "unsuccessful task",
			fields: fields{
				Sequencer: &MockSuccessfulSequencer{},
				Runtime:   &MockRuntime{},
				semaphore: 0,
			},
			args: args{
				t: &MockUnsuccessfulTask{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				Sequencer: tt.fields.Sequencer,
				Runtime:   tt.fields.Runtime,
				semaphore: tt.fields.semaphore,
			}

			if err := c.runTask(tt.args.t); (err != nil) != tt.wantErr {
				t.Errorf("Controller.runTask() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestController_TryLock(t *testing.T) {
	type fields struct {
		Sequencer Sequencer
		Runtime   Runtime
		semaphore int32
	}

	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "is locked",
			fields: fields{
				Sequencer: &MockSuccessfulSequencer{},
				semaphore: 0,
			},
			want: false,
		},
		{
			name: "is unlocked",
			fields: fields{
				Sequencer: &MockSuccessfulSequencer{},
				semaphore: 1,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				Sequencer: tt.fields.Sequencer,
				Runtime:   tt.fields.Runtime,
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
		Sequencer Sequencer
		Runtime   Runtime
		semaphore int32
	}

	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "did not unlock",
			fields: fields{
				semaphore: 0,
			},
			want: false,
		},
		{
			name: "did unlock",
			fields: fields{
				semaphore: 1,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				Sequencer: tt.fields.Sequencer,
				Runtime:   tt.fields.Runtime,
				semaphore: tt.fields.semaphore,
			}
			if got := c.Unlock(); got != tt.want {
				t.Errorf("Controller.Unlock() = %v, want %v", got, tt.want)
			}
		})
	}
}
