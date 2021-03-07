// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

// import (
// 	"fmt"
// 	"net"
// 	"testing"

// 	"github.com/talos-systems/go-procfs/procfs"

// 	"github.com/talos-systems/talos/pkg/machinery/api/machine"
// )

// type MockSuccessfulSequencer struct{}

// // Boot is a mock method that overrides the embedded sequencer's Boot method.
// func (s *MockSuccessfulSequencer) Boot() []Phase {
// 	return []Phase{
// 		&MockSuccessfulPhase{},
// 	}
// }

// // Initialize is a mock method that overrides the embedded sequencer's Initialize method.
// func (s *MockSuccessfulSequencer) Initialize() []Phase {
// 	return []Phase{
// 		&MockSuccessfulPhase{},
// 	}
// }

// // Shutdown is a mock method that overrides the embedded sequencer's Shutdown method.
// func (s *MockSuccessfulSequencer) Shutdown() []Phase {
// 	return []Phase{
// 		&MockSuccessfulPhase{},
// 	}
// }

// // Upgrade is a mock method that overrides the embedded sequencer's Upgrade method.
// func (s *MockSuccessfulSequencer) Upgrade(req *machine.UpgradeRequest) []Phase {
// 	return []Phase{
// 		&MockSuccessfulPhase{},
// 	}
// }

// // Reboot is a mock method that overrides the embedded sequencer's Reboot method.
// func (s *MockSuccessfulSequencer) Reboot() []Phase {
// 	return []Phase{
// 		&MockSuccessfulPhase{},
// 	}
// }

// // Reset is a mock method that overrides the embedded sequencer's Reset method.
// func (s *MockSuccessfulSequencer) Reset(req *machine.ResetRequest) []Phase {
// 	return []Phase{
// 		&MockSuccessfulPhase{},
// 	}
// }

// type MockUnsuccessfulSequencer struct{}

// // Boot is a mock method that overrides the embedded sequencer's Boot method.
// func (s *MockUnsuccessfulSequencer) Boot() []Phase {
// 	return []Phase{
// 		&MockUnsuccessfulPhase{},
// 	}
// }

// // Initialize is a mock method that overrides the embedded sequencer's Initialize method.
// func (s *MockUnsuccessfulSequencer) Initialize() []Phase {
// 	return []Phase{
// 		&MockUnsuccessfulPhase{},
// 	}
// }

// // Shutdown is a mock method that overrides the embedded sequencer's Shutdown method.
// func (s *MockUnsuccessfulSequencer) Shutdown() []Phase {
// 	return []Phase{
// 		&MockUnsuccessfulPhase{},
// 	}
// }

// // Upgrade is a mock method that overrides the embedded sequencer's Upgrade method.
// func (s *MockUnsuccessfulSequencer) Upgrade(req *machine.UpgradeRequest) []Phase {
// 	return []Phase{
// 		&MockUnsuccessfulPhase{},
// 	}
// }

// // Reboot is a mock method that overrides the embedded sequencer's Reboot method.
// func (s *MockUnsuccessfulSequencer) Reboot() []Phase {
// 	return []Phase{
// 		&MockUnsuccessfulPhase{},
// 	}
// }

// // Reset is a mock method that overrides the embedded sequencer's Reset method.
// func (s *MockUnsuccessfulSequencer) Reset(req *machine.ResetRequest) []Phase {
// 	return []Phase{
// 		&MockUnsuccessfulPhase{},
// 	}
// }

// type MockSuccessfulPhase struct{}

// func (*MockSuccessfulPhase) Tasks() []TaskSetupFunc {
// 	return []TaskSetupFunc{&MockSuccessfulTask{}}
// }

// type MockUnsuccessfulPhase struct{}

// func (*MockUnsuccessfulPhase) Tasks() []TaskSetupFunc {
// 	return []TaskSetupFunc{&MockUnsuccessfulTask{}}
// }

// type MockSuccessfulTask struct{}

// func (*MockSuccessfulTask) Func(Mode) TaskSetupFunc {
// 	return func(Runtime) error {
// 		return nil
// 	}
// }

// type MockUnsuccessfulTask struct{}

// func (*MockUnsuccessfulTask) Func(Mode) TaskSetupFunc {
// 	return func(Runtime) error { return fmt.Errorf("error") }
// }

// type MockPlatform struct{}

// func (*MockPlatform) Name() string {
// 	return "mock"
// }

// func (*MockPlatform) Configuration() ([]byte, error) {
// 	return nil, nil
// }

// func (*MockPlatform) ExternalIPs() ([]net.IP, error) {
// 	return nil, nil
// }

// func (*MockPlatform) Hostname() ([]byte, error) {
// 	return nil, nil
// }

// func (*MockPlatform) Mode() Mode {
// 	return Metal
// }

// func (*MockPlatform) KernelArgs() procfs.Parameters {
// 	return procfs.Parameters{}
// }

// type MockConfigurator struct{}

// func (*MockConfigurator) Version() string {
// 	return ""
// }

// func (*MockConfigurator) Debug() bool {
// 	return false
// }

// func (*MockConfigurator) Persist() bool {
// 	return false
// }

// func (*MockConfigurator) Machine() Machine {
// 	return nil
// }

// func (*MockConfigurator) Cluster() Cluster {
// 	return nil
// }

// func (*MockConfigurator) Validate(Mode) error {
// 	return nil
// }

// func (*MockConfigurator) String() (string, error) {
// 	return "", nil
// }

// func (*MockConfigurator) Bytes() ([]byte, error) {
// 	return nil, nil
// }

// type MockRuntime struct{}

// func (*MockRuntime) Platform() Platform {
// 	return &MockPlatform{}
// }

// func (*MockRuntime) Config() Configurator {
// 	return &MockConfigurator{}
// }

// func (*MockRuntime) Sequence() Sequence {
// 	return Noop
// }

// func TestController_Run(t *testing.T) {
// 	type fields struct {
// 		Sequencer Sequencer
// 		Runtime   Runtime
// 		semaphore int32
// 	}

// 	type args struct {
// 		seq  Sequence
// 		data interface{}
// 	}

// 	tests := []struct {
// 		name    string
// 		fields  fields
// 		args    args
// 		wantErr bool
// 	}{
// 		{
// 			name: "boot",
// 			fields: fields{
// 				Sequencer: &MockSuccessfulSequencer{},
// 				Runtime:   &MockRuntime{},
// 				semaphore: 0,
// 			},
// 			args: args{
// 				seq:  Boot,
// 				data: nil,
// 			},
// 			wantErr: false,
// 		},
// 		{
// 			name: "initialize",
// 			fields: fields{
// 				Sequencer: &MockSuccessfulSequencer{},
// 				Runtime:   &MockRuntime{},
// 				semaphore: 0,
// 			},
// 			args: args{
// 				seq:  Initialize,
// 				data: nil,
// 			},
// 			wantErr: false,
// 		},
// 		{
// 			name: "shutdown",
// 			fields: fields{
// 				Sequencer: &MockSuccessfulSequencer{},
// 				Runtime:   &MockRuntime{},
// 				semaphore: 0,
// 			},
// 			args: args{
// 				seq:  Shutdown,
// 				data: nil,
// 			},
// 			wantErr: false,
// 		},
// 		{
// 			name: "upgrade with valid data",
// 			fields: fields{
// 				Sequencer: &MockSuccessfulSequencer{},
// 				Runtime:   &MockRuntime{},
// 				semaphore: 0,
// 			},
// 			args: args{
// 				seq:  Upgrade,
// 				data: &machine.UpgradeRequest{},
// 			},
// 			wantErr: false,
// 		},
// 		{
// 			name: "upgrade with invalid data",
// 			fields: fields{
// 				Sequencer: &MockSuccessfulSequencer{},
// 				Runtime:   &MockRuntime{},
// 				semaphore: 0,
// 			},
// 			args: args{
// 				seq:  Upgrade,
// 				data: nil,
// 			},
// 			wantErr: true,
// 		},
// 		{
// 			name: "upgrade with lock",
// 			fields: fields{
// 				Sequencer: &MockSuccessfulSequencer{},
// 				Runtime:   &MockRuntime{},
// 				semaphore: 1,
// 			},
// 			args: args{
// 				seq:  Upgrade,
// 				data: &machine.UpgradeRequest{},
// 			},
// 			wantErr: true,
// 		},
// 		{
// 			name: "reset with valid data",
// 			fields: fields{
// 				Sequencer: &MockSuccessfulSequencer{},
// 				Runtime:   &MockRuntime{},
// 				semaphore: 0,
// 			},
// 			args: args{
// 				seq:  Reset,
// 				data: &machine.ResetRequest{},
// 			},
// 			wantErr: false,
// 		},
// 		{
// 			name: "reset with invalid data",
// 			fields: fields{
// 				Sequencer: &MockSuccessfulSequencer{},
// 				Runtime:   &MockRuntime{},
// 				semaphore: 0,
// 			},
// 			args: args{
// 				seq:  Reset,
// 				data: nil,
// 			},
// 			wantErr: true,
// 		},
// 		{
// 			name: "unsuccessful phase",
// 			fields: fields{
// 				Sequencer: &MockUnsuccessfulSequencer{},
// 				Runtime:   &MockRuntime{},
// 				semaphore: 0,
// 			},
// 			args: args{
// 				seq:  Boot,
// 				data: nil,
// 			},
// 			wantErr: true,
// 		},
// 		{
// 			name: "undefined runtime",
// 			fields: fields{
// 				Sequencer: &MockSuccessfulSequencer{},
// 				Runtime:   nil,
// 				semaphore: 0,
// 			},
// 			args: args{
// 				seq:  Boot,
// 				data: nil,
// 			},
// 			wantErr: true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			c := &Controller{
// 				Sequencer: tt.fields.Sequencer,
// 				Runtime:   tt.fields.Runtime,
// 				semaphore: tt.fields.semaphore,
// 			}
// 			t.Logf("c.Sequencer: %v", c.Sequencer)
// 			if err := c.Run(tt.args.seq, tt.args.data); (err != nil) != tt.wantErr {
// 				t.Errorf("Controller.Run() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

// func TestController_runPhase(t *testing.T) {
// 	type fields struct {
// 		Sequencer Sequencer
// 		Runtime   Runtime
// 		semaphore int32
// 	}

// 	type args struct {
// 		phase Phase
// 	}

// 	tests := []struct {
// 		name    string
// 		fields  fields
// 		args    args
// 		wantErr bool
// 	}{
// 		{
// 			name: "successful phase",
// 			fields: fields{
// 				Sequencer: &MockSuccessfulSequencer{},
// 				Runtime:   &MockRuntime{},
// 				semaphore: 0,
// 			},
// 			args: args{
// 				phase: &MockSuccessfulPhase{},
// 			},
// 			wantErr: false,
// 		},
// 		{
// 			name: "unsuccessful phase",
// 			fields: fields{
// 				Sequencer: &MockSuccessfulSequencer{},
// 				Runtime:   &MockRuntime{},
// 				semaphore: 0,
// 			},
// 			args: args{
// 				phase: &MockUnsuccessfulPhase{},
// 			},
// 			wantErr: true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			c := &Controller{
// 				Sequencer: tt.fields.Sequencer,
// 				Runtime:   tt.fields.Runtime,
// 				semaphore: tt.fields.semaphore,
// 			}
// 			if err := c.runPhase(tt.args.phase); (err != nil) != tt.wantErr {
// 				t.Errorf("Controller.runPhase() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

// func TestController_runTask(t *testing.T) {
// 	type fields struct {
// 		Sequencer Sequencer
// 		Runtime   Runtime
// 		semaphore int32
// 	}

// 	type args struct {
// 		t TaskSetupFunc
// 	}

// 	tests := []struct {
// 		name    string
// 		fields  fields
// 		args    args
// 		wantErr bool
// 	}{
// 		{
// 			name: "successful task",
// 			fields: fields{
// 				Sequencer: &MockSuccessfulSequencer{},
// 				Runtime:   &MockRuntime{},
// 				semaphore: 0,
// 			},
// 			args: args{
// 				t: &MockSuccessfulTask{},
// 			},
// 			wantErr: false,
// 		},
// 		{
// 			name: "unsuccessful task",
// 			fields: fields{
// 				Sequencer: &MockSuccessfulSequencer{},
// 				Runtime:   &MockRuntime{},
// 				semaphore: 0,
// 			},
// 			args: args{
// 				t: &MockUnsuccessfulTask{},
// 			},
// 			wantErr: true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			c := &Controller{
// 				Sequencer: tt.fields.Sequencer,
// 				Runtime:   tt.fields.Runtime,
// 				semaphore: tt.fields.semaphore,
// 			}

// 			if err := c.runTask(tt.args.t); (err != nil) != tt.wantErr {
// 				t.Errorf("Controller.runTask() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

// func TestController_TryLock(t *testing.T) {
// 	type fields struct {
// 		Sequencer Sequencer
// 		Runtime   Runtime
// 		semaphore int32
// 	}

// 	tests := []struct {
// 		name   string
// 		fields fields
// 		want   bool
// 	}{
// 		{
// 			name: "is locked",
// 			fields: fields{
// 				Sequencer: &MockSuccessfulSequencer{},
// 				semaphore: 0,
// 			},
// 			want: false,
// 		},
// 		{
// 			name: "is unlocked",
// 			fields: fields{
// 				Sequencer: &MockSuccessfulSequencer{},
// 				semaphore: 1,
// 			},
// 			want: true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			c := &Controller{
// 				Sequencer: tt.fields.Sequencer,
// 				Runtime:   tt.fields.Runtime,
// 				semaphore: tt.fields.semaphore,
// 			}
// 			if got := c.TryLock(); got != tt.want {
// 				t.Errorf("Controller.TryLock() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

// func TestController_Unlock(t *testing.T) {
// 	type fields struct {
// 		Sequencer Sequencer
// 		Runtime   Runtime
// 		semaphore int32
// 	}

// 	tests := []struct {
// 		name   string
// 		fields fields
// 		want   bool
// 	}{
// 		{
// 			name: "did not unlock",
// 			fields: fields{
// 				semaphore: 0,
// 			},
// 			want: false,
// 		},
// 		{
// 			name: "did unlock",
// 			fields: fields{
// 				semaphore: 1,
// 			},
// 			want: true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			c := &Controller{
// 				Sequencer: tt.fields.Sequencer,
// 				Runtime:   tt.fields.Runtime,
// 				semaphore: tt.fields.semaphore,
// 			}
// 			if got := c.Unlock(); got != tt.want {
// 				t.Errorf("Controller.Unlock() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

// import (
// 	"errors"
// 	"os"
// 	"testing"

// 	"github.com/stretchr/testify/suite"

// 	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
// 	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/phase"
// )

// type PhaseSuite struct {
// 	suite.Suite

// 	platformExists bool
// 	platformValue  string
// }

// type regularTask struct {
// 	errCh <-chan error
// }

// func (t *regularTask) TaskFunc(runtime.Mode) TaskFunc {
// 	return func(runtime.Runtime) error {
// 		return <-t.errCh
// 	}
// }

// type nilTask struct{}

// func (t *nilTask) TaskFunc(runtime.Mode) TaskFunc {
// 	return nil
// }

// type panicTask struct{}

// func (t *panicTask) TaskFunc(runtime.Mode) TaskFunc {
// 	return func(runtime.Runtime) error {
// 		panic("in task")
// 	}
// }

// func (suite *PhaseSuite) SetupSuite() {
// 	suite.platformValue, suite.platformExists = os.LookupEnv("PLATFORM")
// 	suite.Require().NoError(os.Setenv("PLATFORM", "container"))
// }

// func (suite *PhaseSuite) TearDownSuite() {
// 	if !suite.platformExists {
// 		suite.Require().NoError(os.Unsetenv("PLATFORM"))
// 	} else {
// 		suite.Require().NoError(os.Setenv("PLATFORM", suite.platformValue))
// 	}
// }

// func (suite *PhaseSuite) TestRunSuccess() {
// 	r, err := phase.NewRunner(nil, runtime.Noop)
// 	suite.Require().NoError(err)

// 	taskErr := make(chan error)

// 	r.Add(phase.NewPhase("empty"))
// 	r.Add(phase.NewPhase("phase1", &regularTask{errCh: taskErr}, &regularTask{errCh: taskErr}))
// 	r.Add(phase.NewPhase("phase2", &regularTask{errCh: taskErr}, &nilTask{}))

// 	errCh := make(chan error)

// 	go func() {
// 		errCh <- r.Run()
// 	}()

// 	taskErr <- nil
// 	taskErr <- nil

// 	select {
// 	case <-errCh:
// 		suite.Require().Fail("should be still running")
// 	default:
// 	}

// 	taskErr <- nil

// 	suite.Require().NoError(<-errCh)
// }

// func (suite *PhaseSuite) TestRunFailures() {
// 	r, err := phase.NewRunner(nil, runtime.Noop)
// 	suite.Require().NoError(err)

// 	taskErr := make(chan error, 1)

// 	r.Add(phase.NewPhase("empty"))
// 	r.Add(phase.NewPhase("failphase", &panicTask{}, &regularTask{errCh: taskErr}, &nilTask{}))
// 	r.Add(phase.NewPhase("neverreached",
// 		&regularTask{}, // should never be reached
// 	))

// 	taskErr <- errors.New("test error")

// 	err = r.Run()
// 	suite.Require().Error(err)
// 	suite.Assert().Contains(err.Error(), "2 errors occurred")
// 	suite.Assert().Contains(err.Error(), "test error")
// 	suite.Assert().Contains(err.Error(), "panic recovered: in task")
// }

// func TestPhaseSuite(t *testing.T) {
// 	suite.Run(t, new(PhaseSuite))
// }
